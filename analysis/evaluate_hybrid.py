#!/usr/bin/env python3
"""Evaluate the hybrid offline model on the same cutoffs.

Replicates the serving hybrid: co-purchase lift + popularity + title overlap.
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
CUTOFFS = ["2026-05-01", "2026-07-01", "2026-09-01"]
STOP = {"для", "или", "при", "лицензия", "версия", "edition", "renewal", "russian"}


def tokens(name):
    return {
        w for w in re.split(r"[^a-zа-я0-9]+", (name or "").lower())
        if len(w) >= 4 and w not in STOP
    }


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("SELECT item_id, title, available FROM lk_items")
    items = {r[0]: {"title": r[1], "available": r[2], "tokens": tokens(r[1])} for r in cur.fetchall()}

    cur.execute("SELECT client_id, client_type, COALESCE(industry, '') FROM lk_clients")
    clients = {}
    for cid, ctype, industry in cur.fetchall():
        clients[cid] = {
            "audience": "b2c" if (ctype or "").upper() == "ФЛ" else "b2b",
            "industry": industry,
        }

    cur.execute("SELECT client_id, order_id, item_id, order_week::text FROM lk_orders")
    orders = cur.fetchall()

    report = []
    for cutoff in CUTOFFS:
        pre = defaultdict(set)
        post = defaultdict(set)
        baskets = defaultdict(set)
        for cid, oid, item_id, week in orders:
            if week < cutoff:
                pre[cid].add(item_id)
                baskets[(oid, week)].add(item_id)
            else:
                post[cid].add(item_id)

        pop = Counter()
        item_pop = Counter()
        for cid, oid, item_id, week in orders:
            if week >= cutoff:
                continue
            c = clients.get(cid)
            if c and c["audience"] == "b2b":
                pop[item_id] += 1
        for (oid, week), basket in baskets.items():
            for item in basket:
                item_pop[item] += 1
        neighbours = defaultdict(list)
        pair_counts = Counter()
        for basket in baskets.values():
            for a in basket:
                for b in basket:
                    if a != b:
                        pair_counts[(a, b)] += 1
        for (a, b), count in pair_counts.items():
            neighbours[a].append((b, count))
        for a in neighbours:
            neighbours[a].sort(key=lambda x: (-x[1], x[0]))

        max_pop = max(pop.values()) if pop else 1
        pop_list = [i for i, _ in pop.most_common()]

        def recommend(client_id):
            c = clients.get(client_id)
            if not c:
                return []
            owned = pre[client_id]
            if c["audience"] == "b2c":
                return [i for i in pop_list if i not in owned][:3]
            scores = defaultdict(float)
            for a in owned:
                for b, count in neighbours.get(a, [])[:200]:
                    if b in owned:
                        continue
                    scores[b] += 3.0 * count / math.sqrt(
                        max(1, item_pop[a]) * max(1, item_pop[b])
                    )
            for b, count in pop.items():
                if b not in owned:
                    scores[b] += 0.5 * count / max_pop
            owned_tokens = set()
            for a in owned:
                owned_tokens |= items.get(a, {}).get("tokens", set())
            for b in pop_list[:500]:
                if b in owned:
                    continue
                overlap = len(owned_tokens & items.get(b, {}).get("tokens", set()))
                if overlap:
                    scores[b] += 0.2 * overlap
            ranked = [i for i, _ in sorted(scores.items(), key=lambda kv: (-kv[1], kv[0])) if items.get(i, {}).get("available")]
            if len(ranked) < 3:
                ranked += [i for i in pop_list if i not in owned and i not in ranked and items.get(i, {}).get("available")]
            return ranked[:3]

        hits = 0
        p3 = 0.0
        n = 0
        for cid, owned in pre.items():
            target = post.get(cid)
            if not owned or not target:
                continue
            n += 1
            recs = recommend(cid)
            h = sum(1 for i in recs if i in target)
            if h:
                hits += 1
            p3 += h / 3
        report.append({
            "cutoff": cutoff,
            "clients": n,
            "hit_rate": round(hits / n, 4) if n else 0,
            "p_at_3": round(p3 / n, 4) if n else 0,
        })
        print(report[-1])

    out = os.path.join(os.path.dirname(__file__), "hybrid_evaluation.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=2)
    print("saved", out)


if __name__ == "__main__":
    main()
