#!/usr/bin/env python3
"""Incrementally upsert catalog positions without touching existing relations.

Examples:
    python3 analysis/import_items.py --csv new_positions.csv
    python3 analysis/import_items.py --dataset v6.xlsx
    python3 analysis/import_items.py --dataset v6.xlsx --orders
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

import pandas as pd
import psycopg2
from psycopg2.extras import execute_values

sys.path.insert(0, str(Path(__file__).resolve().parent))
from import_dataset import category_for  # noqa: E402


def truthy(value: object) -> bool:
    return str(value).strip().lower() in {"true", "1", "yes", "да"}


def normalize_items(frame: pd.DataFrame, source: str) -> pd.DataFrame:
    frame = frame.copy()
    if "item_name" in frame.columns and "title" not in frame.columns:
        frame = frame.rename(columns={"item_name": "title"})
    for column in ("item_id", "sku", "title", "product_group", "category", "subcategory"):
        if column not in frame.columns:
            frame[column] = ""
        frame[column] = frame[column].fillna("").astype(str).str.strip()
    if "available" not in frame.columns:
        frame["available"] = "true"
    frame["available"] = frame["available"].map(truthy)
    frame = frame[frame["item_id"] != ""]
    frame["category"] = [
        category or category_for(title) for category, title in zip(frame["category"], frame["title"])
    ]
    frame["source"] = source
    return frame.drop_duplicates("item_id", keep="last")


def read_items(args: argparse.Namespace) -> pd.DataFrame:
    if args.csv:
        return normalize_items(pd.read_csv(args.csv, dtype=str), "import_items")
    sales = pd.read_excel(args.dataset, sheet_name="Продажи", dtype=str)
    items = sales[["item_id", "sku", "item_name", "product_group"]].copy()
    if "available" in sales.columns:
        items["available"] = sales["available"]
    return normalize_items(items, "dataset_v5")


def upsert_items(cur, items: pd.DataFrame) -> tuple[int, int]:
    ids = items["item_id"].tolist()
    cur.execute("SELECT item_id FROM lk_items WHERE item_id = ANY(%s)", (ids,))
    existing = {row[0] for row in cur.fetchall()}
    execute_values(cur, """
        INSERT INTO lk_items
            (item_id, sku, title, category, subcategory, product_group, available, source)
        VALUES %s
        ON CONFLICT (item_id) DO UPDATE SET
            sku = EXCLUDED.sku,
            title = EXCLUDED.title,
            category = EXCLUDED.category,
            subcategory = COALESCE(NULLIF(EXCLUDED.subcategory, ''), lk_items.subcategory),
            product_group = EXCLUDED.product_group,
            available = EXCLUDED.available
    """, [
        (r.item_id, r.sku, r.title, r.category, r.subcategory,
         r.product_group, bool(r.available), r.source)
        for r in items.itertuples()
    ], page_size=1000)
    added = len(set(ids) - existing)
    updated = len(set(ids) & existing)
    return added, updated


def import_orders(cur, path: str) -> tuple[int, int]:
    sales = pd.read_excel(path, sheet_name="Продажи", dtype=str)
    clients = pd.read_excel(path, sheet_name="Клиенты", dtype=str).fillna("")
    sales = sales.drop_duplicates(["order_id", "order_week", "item_id"]).copy()
    sales["order_week"] = pd.to_datetime(sales["order_week"]).dt.date
    execute_values(cur, """
        INSERT INTO lk_clients
            (client_id, client_group, client_type, industry, segment, lk_access, source)
        VALUES %s
        ON CONFLICT (client_id) DO UPDATE SET
            client_group = EXCLUDED.client_group,
            client_type = EXCLUDED.client_type,
            industry = EXCLUDED.industry,
            segment = EXCLUDED.segment,
            lk_access = EXCLUDED.lk_access
    """, [
        (r.client_id, r.client_group, r.client_type, r.industry,
         r.segment, r.lk_access, "dataset_v5")
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

    cur.execute("TRUNCATE co_purchase_pairs")
    cur.execute("""
        INSERT INTO co_purchase_pairs (item_a, item_b, count, b2b_count, b2c_count, source)
        SELECT a.item_id, b.item_id, count(*),
               count(*) FILTER (WHERE c.client_group = 'B2B'),
               count(*) FILTER (WHERE c.client_group = 'B2C'),
               'incremental'
        FROM lk_orders a
        JOIN lk_orders b
          ON b.order_id = a.order_id
         AND b.order_week = a.order_week
         AND a.item_id < b.item_id
        JOIN lk_clients c ON c.client_id = a.client_id
        GROUP BY 1, 2
    """)
    cur.execute("SELECT count(*) FROM co_purchase_pairs")
    pairs = cur.fetchone()[0]
    cur.execute("SELECT count(DISTINCT order_id) FROM lk_orders")
    orders = cur.fetchone()[0]
    return orders, pairs


def main() -> None:
    ap = argparse.ArgumentParser()
    source = ap.add_mutually_exclusive_group(required=True)
    source.add_argument("--csv", help="CSV with item_id,title,sku,product_group,available,category")
    source.add_argument("--dataset", help="XLSX with a Продажи sheet")
    ap.add_argument("--orders", action="store_true",
                    help="also upsert clients/orders and rebuild co-purchase pairs")
    ap.add_argument("--dsn", default=os.getenv(
        "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"))
    args = ap.parse_args()

    items = read_items(args)
    print("positions in file:", len(items))

    conn = psycopg2.connect(args.dsn)
    conn.autocommit = False
    try:
        with conn.cursor() as cur:
            added, updated = upsert_items(cur, items)
            orders = pairs = None
            if args.orders:
                if not args.dataset:
                    raise SystemExit("--orders requires --dataset with a Клиенты sheet")
                orders, pairs = import_orders(cur, args.dataset)
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()

    print("items added:", added, "updated:", updated)
    if orders is not None:
        print("orders:", orders, "co-purchase pairs:", pairs)
        print("now press «Сгенерировать» in the Catalog tab to refresh candidates")


if __name__ == "__main__":
    main()
