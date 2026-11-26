#!/usr/bin/env python3
"""Import the LK v3 dataset into PostgreSQL and build the co-purchase graph."""

from __future__ import annotations

import argparse
import itertools
import os
from collections import Counter, defaultdict
from pathlib import Path

import pandas as pd
import psycopg2
from psycopg2.extras import execute_values


def category_for(title: str) -> str:
    t = title.lower()
    rules = [
        ("games", ["game", "игр", "gamepad"]),
        ("security", ["антивирус", "kaspersky", "dr.web", "drweb", "security", "крипто", "криптопро", "магпро", "фстэк", "fstec", "инфобезопас"]),
        ("backup", ["backup", "бэкап", "recovery", "disaster", "кибербэкап"]),
        ("office", ["office", "microsoft 365", "m365", "windows", "word", "excel", "powerpoint"]),
        ("graphics", ["corel", "adobe", "cad", "компас", "nanocad", "visio", "graphics", "photo", "raster", "vector", "design", "лира", "renga"]),
        ("devtools", ["ide", " ci", "git", "navicat", "postgres", "odbc", "provider", "sdk", "atlas", "monitoring", "devtools", "api"]),
        ("training", ["курс", "обучен", "сертификац", "тренинг", "course"]),
    ]
    for category, keys in rules:
        if any(k in t for k in keys):
            return category
    return "other"


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dataset", default=os.getenv("DATASET", "/home/basili4/Загрузки/Датасет_Рекомендации_ЛК_v3.xlsx"))
    ap.add_argument("--dsn", default=os.getenv("DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"))
    args = ap.parse_args()

    path = Path(args.dataset)
    print("reading", path)
    sales = pd.read_excel(path, sheet_name="Продажи", dtype=str)
    clients = pd.read_excel(path, sheet_name="Клиенты", dtype=str)

    sales = sales.drop_duplicates(["order_id", "item_id"]).copy()
    sales["order_week"] = pd.to_datetime(sales["order_week"]).dt.date
    sales["item_id"] = sales["item_id"].str.strip()
    sales["item_name"] = sales["item_name"].fillna("").str.strip()
    sales["sku"] = sales["sku"].fillna("").str.strip()
    clients = clients.fillna("")

    items = (
        sales[["item_id", "sku", "item_name"]]
        .drop_duplicates("item_id")
        .rename(columns={"item_name": "title"})
    )
    items["category"] = items["title"].map(category_for)
    items["subcategory"] = ""

    # Co-purchase pairs from unique multi-item orders.
    client_group = dict(zip(clients.client_id, clients.client_group))
    pairs: Counter[tuple[str, str]] = Counter()
    pair_b2b: Counter[tuple[str, str]] = Counter()
    pair_b2c: Counter[tuple[str, str]] = Counter()
    for order_id, group in sales.groupby("order_id"):
        ids = sorted(set(group.item_id))
        if len(ids) < 2:
            continue
        cid = group.client_id.iloc[0]
        grp = client_group.get(cid, "")
        for a, b in itertools.combinations(ids, 2):
            pairs[(a, b)] += 1
            if grp == "B2B":
                pair_b2b[(a, b)] += 1
            elif grp == "B2C":
                pair_b2c[(a, b)] += 1

    conn = psycopg2.connect(args.dsn)
    conn.autocommit = False
    try:
        with conn.cursor() as cur:
            cur.execute("""
                TRUNCATE lk_relation_candidates, co_purchase_pairs, lk_orders,
                         lk_clients, lk_items RESTART IDENTITY CASCADE
            """)
            execute_values(cur, """
                INSERT INTO lk_items (item_id, sku, title, category, subcategory, source)
                VALUES %s
            """, [
                (r.item_id, r.sku, r.title, r.category, r.subcategory, "dataset_v3")
                for r in items.itertuples()
            ], page_size=1000)

            execute_values(cur, """
                INSERT INTO lk_clients (client_id, client_group, client_type, industry, segment, lk_access, source)
                VALUES %s
            """, [
                (r.client_id, r.client_group, r.client_type, r.industry, r.segment, r.lk_access, "dataset_v3")
                for r in clients.itertuples()
            ], page_size=1000)

            execute_values(cur, """
                INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
                VALUES %s
                ON CONFLICT DO NOTHING
            """, [
                (r.order_id, r.client_id, r.order_week, r.item_id)
                for r in sales.itertuples()
            ], page_size=2000)

            execute_values(cur, """
                INSERT INTO co_purchase_pairs (item_a, item_b, count, b2b_count, b2c_count, source)
                VALUES %s
                ON CONFLICT (item_a, item_b) DO UPDATE SET
                    count = EXCLUDED.count,
                    b2b_count = EXCLUDED.b2b_count,
                    b2c_count = EXCLUDED.b2c_count,
                    updated_at = now()
            """, [
                (a, b, n, pair_b2b[(a, b)], pair_b2c[(a, b)], "dataset_v3")
                for (a, b), n in pairs.items()
            ], page_size=1000)
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()

    covered = len({x for pair in pairs for x in pair})
    print("imported:")
    print("  items", len(items))
    print("  clients", len(clients))
    print("  orders", sales.order_id.nunique())
    print("  sales rows", len(sales))
    print("  co-purchase pairs", len(pairs))
    print("  pairs count>=2", sum(1 for n in pairs.values() if n >= 2))
    print("  items in pairs", covered)
    print("  solo items", len(items) - covered)


if __name__ == "__main__":
    main()
