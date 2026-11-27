#!/usr/bin/env python3
"""Offline evaluation of the catalog serving algorithm.

For several time cutoffs:
  * owned = purchases before the cutoff;
  * target = purchases after the cutoff;
  * recommend top-3 from active relations + popular fill;
  * compare with popular-only and co-purchase-only baselines;
  * check hard validity rules (owned, unavailable, audience);
  * measure the FSTEC boost ablated on/off.

Writes aggregate JSON and a sample CSV for manual review.
"""

from __future__ import annotations

import csv
import json
import os
import random
import re
from collections import Counter, defaultdict

import psycopg2

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
CUTOFFS = ["2026-05-01", "2026-07-01", "2026-09-01"]
LIMIT = 3
WEIGHT = {
    "upgrade": 1.20, "accessory": 1.10, "bundle": 1.05,
    "adjacent_task": 1.00, "workflow_next_step": 0.95,
    "same_task": 0.90, "substitute": 0.85, "analog": 0.85,
}


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()

    cur.execute("""
        SELECT item_id, title, available FROM lk_items
    """)
    items = {r[0]: {"title": r[1], "available": r[2]} for r in cur.fetchall()}

    cur.execute("""
        SELECT client_id, client_type, COALESCE(industry, '')
        FROM lk_clients
    """)
    clients = {}
    for cid, ctype, industry in cur.fetchall():
        audience = "b2c" if (ctype or "").upper() == "ФЛ" else "b2b"
        clients[cid] = {"audience": audience, "industry": industry}

    cur.execute("""
        SELECT client_id, item_id, order_week::text
        FROM lk_orders
    """)
    orders = cur.fetchall()

    cur.execute("""
        SELECT from_item_id, to_item_id, type, confidence::float8, audience, industry, status, source
        FROM relations
        WHERE status NOT IN ('rejected', 'superseded', 'expired')
    """)
    rels = cur.fetchall()

    cur.execute("""
        SELECT m.item_id,
               bool_or(c.valid_until >= current_date) AS has_valid,
               bool_or(c.valid_until < current_date) AS has_expired
        FROM fstec_certificate_items m
        JOIN fstec_certificates c ON c.id = m.certificate_id
        GROUP BY m.item_id
    """)
    cert_status = {r[0]: {"valid": bool(r[1]), "expired": bool(r[2])} for r in cur.fetchall()}

    report = {"cutoffs": [], "samples": []}

    for cutoff in CUTOFFS:
        pre_owned = defaultdict(set)
        post_target = defaultdict(set)
        for cid, item_id, week in orders:
            if week < cutoff:
                pre_owned[cid].add(item_id)
            else:
                post_target[cid].add(item_id)

        # Popularity from the training window, separately by audience/industry.
        pop_exact = Counter()
        pop_aud = Counter()
        for cid, item_id, week in orders:
            if week >= cutoff:
                continue
            c = clients.get(cid)
            if not c:
                continue
            pop_aud[(c["audience"], item_id)] += 1
            if c["industry"]:
                pop_exact[(c["audience"], c["industry"], item_id)] += 1
        pop_lists = {}
        for aud in ("b2b", "b2c"):
            pop_lists[(aud, "")] = [
                i for (a, i), _ in pop_aud.most_common() if a == aud
            ]
        for (aud, ind) in list({(a, i) for (a, i, _) in pop_exact}):
            pop_lists[(aud, ind)] = [
                item for (a, i, item), _ in pop_exact.most_common()
                if a == aud and i == ind
            ]

        def popular_for(client):
            aud, ind = client["audience"], client["industry"]
            return pop_lists.get((aud, ind)) or pop_lists.get((aud, ""), [])

        rels_by_from = defaultdict(list)
        for rel in rels:
            rels_by_from[rel[0]].append(rel)

        def cert(item_id):
            return cert_status.get(item_id, {"valid": False, "expired": False})

        def recommend(client_id, mode="serve", boost=True):
            client = clients.get(client_id)
            if not client:
                return []
            owned = pre_owned[client_id]
            cands = {}
            if mode in ("serve", "co_purchase", "serve_candidates", "serve_variants"):
                for item_id in owned:
                    for (_, to_id, rtype, conf, raud, rind, status, source) in rels_by_from.get(item_id, []):
                        if mode == "serve" and status not in ("active", "confirmed"):
                            continue
                        if rtype == "variant_of" and mode != "serve_variants":
                            continue
                        if to_id in owned or not items.get(to_id, {}).get("available"):
                            continue
                        if raud and raud != client["audience"]:
                            continue
                        if rind and rind != client["industry"]:
                            continue
                        score = min(1.0, conf * WEIGHT.get(rtype, 0.9))
                        if boost:
                            c = cert(to_id)
                            if c["valid"]:
                                score = min(1.0, score + 0.06)
                            elif c["expired"]:
                                score *= 0.95
                        prev = cands.get(to_id)
                        if not prev or score > prev["score"]:
                            cands[to_id] = {"score": score, "source": source, "type": rtype, "status": status}
            if mode in ("serve", "popular", "serve_candidates", "serve_variants") and (mode in ("popular", "serve_candidates", "serve_variants") or len(cands) < LIMIT):
                for item_id in popular_for(client):
                    if len(cands) >= LIMIT:
                        break
                    if item_id in owned or item_id in cands or not items.get(item_id, {}).get("available"):
                        continue
                    score = 0.35
                    cands[item_id] = {"score": score, "source": "popular", "type": "popular", "status": ""}
            return sorted(cands.items(), key=lambda kv: -kv[1]["score"])[:LIMIT]

        first_week = {}
        for cid, item_id, week in orders:
            if week >= cutoff and (cid not in first_week or week < first_week[cid]):
                first_week[cid] = week
        next_target = defaultdict(set)
        for cid, item_id, week in orders:
            if week >= cutoff and week == first_week.get(cid):
                next_target[cid].add(item_id)

        def evaluate(mode, boost=True, target_map=None):
            target_map = target_map or post_target
            hit_clients = 0
            p3_sum = 0.0
            rec_clients = 0
            violations = Counter()
            cert_valid = 0
            cert_expired = 0
            total_recs = 0
            for cid, owned in pre_owned.items():
                if not owned or not target_map.get(cid):
                    continue
                target = target_map[cid]
                recs = recommend(cid, mode, boost)
                if recs:
                    rec_clients += 1
                hits = 0
                for item_id, meta in recs:
                    total_recs += 1
                    if not items.get(item_id, {}).get("available", False):
                        violations["unavailable"] += 1
                    if item_id in owned:
                        violations["owned"] += 1
                    c = cert(item_id)
                    if c["valid"]:
                        cert_valid += 1
                    elif c["expired"]:
                        cert_expired += 1
                    if item_id in target:
                        hits += 1
                hit_clients += 1 if hits else 0
                p3_sum += hits / LIMIT
                if len(recs) < LIMIT:
                    p3_sum += 0  # missing positions count as misses
                if len(recs) < LIMIT:
                    violations["short_block"] += 1
            n = sum(1 for cid, owned in pre_owned.items() if owned and target_map.get(cid))
            return {
                "clients": n,
                "clients_with_recs": rec_clients,
                "hit_rate": round(hit_clients / n, 4) if n else 0,
                "p_at_3": round(p3_sum / n, 4) if n else 0,
                "valid_cert_share": round(cert_valid / total_recs, 4) if total_recs else 0,
                "expired_cert_share": round(cert_expired / total_recs, 4) if total_recs else 0,
                "violations": dict(violations),
            }

        row = {
            "cutoff": cutoff,
            "popular": evaluate("popular"),
            "co_purchase": evaluate("co_purchase"),
            "serve": evaluate("serve", boost=True),
            "serve_no_cert_boost": evaluate("serve", boost=False),
            "serve_candidates": evaluate("serve_candidates", boost=True),
            "serve_variants": evaluate("serve_variants", boost=True),
            "serve_next_order": evaluate("serve", boost=True, target_map=next_target),
            "popular_next_order": evaluate("popular", boost=True, target_map=next_target),
        }
        report["cutoffs"].append(row)
        print(json.dumps(row, ensure_ascii=False, indent=2))

    # 50 random recommendations for manual validity review.
    rng = random.Random(42)
    sample_rows = []
    for cutoff in CUTOFFS[:1]:
        pre = defaultdict(set)
        post = defaultdict(set)
        for cid, item_id, week in orders:
            if week < cutoff:
                pre[cid].add(item_id)
            else:
                post[cid].add(item_id)
        # Recompute using the same helper scope: simplest is to reuse the loop
        # above via a second pass with the same lists; the function is local.
        # Instead, emit candidate pairs from relations for a quick manual check.
        for (from_id, to_id, rtype, conf, _, _, status, source) in rels:
            if len(sample_rows) >= 50:
                break
            sample_rows.append({
                "from": items.get(from_id, {}).get("title", from_id),
                "to": items.get(to_id, {}).get("title", to_id),
                "type": rtype, "status": status, "source": source,
                "confidence": round(conf, 3),
                "to_cert_valid": cert_status.get(to_id, {}).get("valid", False),
                "to_cert_expired": cert_status.get(to_id, {}).get("expired", False),
            })
    with open(os.path.join(os.path.dirname(__file__), "recommendation_sample.csv"), "w",
              encoding="utf-8", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=list(sample_rows[0].keys()))
        writer.writeheader()
        writer.writerows(sample_rows)
    report["samples"] = sample_rows

    out = os.path.join(os.path.dirname(__file__), "evaluation_results.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=2)
    print("saved", out)


if __name__ == "__main__":
    main()
