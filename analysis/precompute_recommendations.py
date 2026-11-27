#!/usr/bin/env python3
"""Precompute offline-model recommendations into client_recommendations.

Uses the same model as analysis/evaluate.py (co-purchase lift + popularity +
title overlap) trained on the full dataset, and stores top-10 per client.
"""

from __future__ import annotations

import argparse
import os
import sys
import time
from pathlib import Path

import psycopg2
from psycopg2.extras import execute_values

sys.path.insert(0, str(Path(__file__).parent))
from evaluate import Model, load  # noqa: E402

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
DEFAULT = "/home/basili4/Загрузки/Датасет_Рекомендации_ЛК_v5.xlsx"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default=DEFAULT)
    parser.add_argument("--top", type=int, default=10)
    parser.add_argument("--train-end", default="", help="optional cutoff for leakage-free evaluation")
    args = parser.parse_args()

    started = time.time()
    sales, clients = load(Path(args.dataset))
    if args.train_end:
        sales = sales[sales["order_week"] <= args.train_end]
        print("train rows", len(sales), "cutoff", args.train_end)
    train_items = sales.groupby("client_id")["item_id"].apply(set).to_dict()
    model = Model(sales, train_items)

    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("SELECT item_id FROM lk_items WHERE available")
    available = {r[0] for r in cur.fetchall()}

    rows = []
    groups = dict(zip(clients["client_id"], clients["client_group"]))
    for idx, (cid, group) in enumerate(groups.items()):
        recs = model.recommend(cid, group, k=args.top * 2)
        recs = [i for i in recs if i in available][: args.top]
        for rank, item_id in enumerate(recs, 1):
            rows.append((cid, item_id, rank, 1.0 / rank, "offline_model",
                         "Подобрано по вашей истории"))
        if idx % 2000 == 0:
            print("clients", idx, "rows", len(rows), flush=True)

    cur.execute("TRUNCATE client_recommendations")
    execute_values(cur, """
        INSERT INTO client_recommendations (client_id, item_id, rank, score, source, reason)
        VALUES %s
        ON CONFLICT DO NOTHING
    """, rows, page_size=2000)
    conn.commit()
    print({"clients": len(groups), "rows": len(rows), "seconds": round(time.time() - started, 1)})
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
