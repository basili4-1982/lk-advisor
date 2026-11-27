#!/usr/bin/env python3
"""Official-protocol evaluation of serving variants.

train <= 2026-08-31, test >= 2026-09-07, target = new test items.
Compares popular, co-purchase, current relation serving and the hybrid model.
"""

from __future__ import annotations

import collections
import json
import math
import os
import re
from collections import Counter, defaultdict

import psycopg2

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
TRAIN_END = "2026-08-31"
TEST_START = "2026-09-07"
STOP = {"для", "или", "при", "лицензия", "версия", "edition", "renewal", "russian", "на", "по", "с", "и", "в"}


def tokens(name):
    return set(
        str(name or "").lower().replace("ё", "е").replace(",", " ").replace(".", " ")
        .replace("-", " ").replace("(", " ").replace(")", " ").split()
    )


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("SELECT item_id, title, available FROM lk_items")
    items = {r[0]: {"title": r[1], "available": r[2], "tokens": tokens(r[1])} for r in cur.fetchall()}

    cur.execute("SELECT client_id, client_type FROM lk_clients")
    clients = {
        cid: {"audience": "b2c" if (ctype or "").upper() == "ФЛ" else "b2b"}
        for cid, ctype in cur.fetchall()
    }

    cur.execute("SELECT client_id, order_id, item_id, order_week::text FROM lk_orders")
    orders = cur.fetchall()

    cur.execute("""
        SELECT from_item_id, to_item_id, type, confidence::float8, audience, industry
        FROM relations WHERE status IN ('active', 'confirmed') AND type <> 'variant_of'
    """)
    rels_by_from = defaultdict(list)
    for row in cur.fetchall():
        rels_by_from[row[0]].append(row)

    cur.execute("""
        SELECT m.item_id, bool_or(c.valid_until >= current_date)
        FROM fstec_certificate_items m
        JOIN fstec_certificates c ON c.id = m.certificate_id
        GROUP BY m.item_id
    """)
    cert_valid = {r[0]: bool(r[1]) for r in cur.fetchall()}

    pre = defaultdict(set)
    test_new = defaultdict(set)
    baskets = defaultdict(set)
    for cid, oid, item_id, week in orders:
        if week <= TRAIN_END:
            pre[cid].add(item_id)
            baskets[(oid, week)].add(item_id)
        elif week >= TEST_START:
            test_new[cid].add(item_id)
    targets = {cid: test_new[cid] - pre.get(cid, set()) for cid in test_new}
    group_a = {cid for cid, new in targets.items() if cid in pre and new}
    group_b = {cid for cid in group_a if len(pre[cid]) == 1}

    pop_sets = defaultdict(set)
    item_pop = Counter()
    for cid, oid, item_id, week in orders:
        if week > TRAIN_END:
            continue
        c = clients.get(cid)
        if c and c["audience"] == "b2b":
            pop_sets[item_id].add(cid)
    pop = Counter({item: len(ids) for item, ids in pop_sets.items()})
    for basket in baskets.values():
        for item in basket:
            item_pop[item] += 1
    pair_counts = Counter()
    for basket in baskets.values():
        for a in basket:
            for b in basket:
                if a != b:
                    pair_counts[(a, b)] += 1
    neighbours = defaultdict(list)
    for (a, b), count in pair_counts.items():
        neighbours[a].append((b, count))
    for a in neighbours:
        neighbours[a].sort(key=lambda x: (-x[1], x[0]))
    max_pop = max(pop.values()) if pop else 1
    pop_list = [i for i, _ in pop.most_common()]

    def popular(client_id):
        owned = pre.get(client_id, set())
        return [i for i in pop_list if i not in owned and items.get(i, {}).get("available")][:3]

    def co_purchase(client_id):
        owned = pre.get(client_id, set())
        scores = defaultdict(float)
        for a in owned:
            for b, count in neighbours.get(a, [])[:200]:
                if b in owned or not items.get(b, {}).get("available"):
                    continue
                scores[b] += 3.0 * count / math.sqrt(max(1, item_pop[a]) * max(1, item_pop[b]))
        return [i for i, _ in sorted(scores.items(), key=lambda kv: (-kv[1], kv[0]))][:3]

    def relations(client_id):
        owned = pre.get(client_id, set())
        c = clients.get(client_id, {})
        scores = defaultdict(float)
        for a in owned:
            for (_, b, rtype, conf, aud, ind) in rels_by_from.get(a, []):
                if b in owned or not items.get(b, {}).get("available"):
                    continue
                if aud and aud != c.get("audience"):
                    continue
                scores[b] += conf
        return [i for i, _ in sorted(scores.items(), key=lambda kv: (-kv[1], kv[0]))][:3]

    def hybrid(client_id, boost=True):
        c = clients.get(client_id, {})
        owned = pre.get(client_id, set())
        if c.get("audience") == "b2c":
            return popular(client_id)
        scores = defaultdict(float)
        for a in owned:
            for b, count in neighbours.get(a, [])[:200]:
                if b in owned or not items.get(b, {}).get("available"):
                    continue
                scores[b] += 3.0 * count / math.sqrt(max(1, item_pop[a]) * max(1, item_pop[b]))
        for b, count in pop.items():
            if b not in owned and items.get(b, {}).get("available"):
                scores[b] += 0.5 * count / max_pop
        for a in owned:
            at = items.get(a, {}).get("tokens", set())
            for b in pop_list[:500]:
                if b in owned or not items.get(b, {}).get("available"):
                    continue
                overlap = len(at & items.get(b, {}).get("tokens", set()))
                if overlap:
                    scores[b] += 0.2 * overlap
        if boost:
            for b in scores:
                if cert_valid.get(b):
                    scores[b] *= 1.06
        return [i for i, _ in sorted(scores.items(), key=lambda kv: (-kv[1], kv[0]))][:3]

    def evaluate(fn, ids):
        hits = 0
        p3 = 0.0
        for cid in ids:
            recs = fn(cid)
            h = sum(1 for i in recs if i in targets[cid])
            hits += 1 if h else 0
            p3 += h / 3
        n = len(ids)
        return {
            "n": n,
            "hit_rate": round(hits / n, 4) if n else 0,
            "p_at_3": round(p3 / n, 4) if n else 0,
        }

    def by_group(fn):
        out = {}
        for name, ids in (("all", group_a), ("b2b", {c for c in group_a if clients[c]["audience"] == "b2b"}),
                          ("b2c", {c for c in group_a if clients[c]["audience"] == "b2c"}),
                          ("B", group_b)):
            out[name] = evaluate(fn, ids)
        return out

    report = {
        "protocol": {"train_end": TRAIN_END, "test_start": TEST_START,
                     "group_a": len(group_a), "group_b": len(group_b)},
        "popular": by_group(popular),
        "co_purchase": by_group(co_purchase),
        "relations": by_group(relations),
        "hybrid": by_group(hybrid),
        "hybrid_no_cert": by_group(lambda cid: hybrid(cid, boost=False)),
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    out = os.path.join(os.path.dirname(__file__), "official_serving_evaluation.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=2)
    print("saved", out)


if __name__ == "__main__":
    main()
