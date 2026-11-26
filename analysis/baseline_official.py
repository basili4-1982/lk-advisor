#!/usr/bin/env python3
"""Official baseline protocol for the LK dataset.

Matches the organisers' rules:
  * train: order_week <= 2026-08-31; test: order_week >= 2026-09-07;
  * basket = (order_id, order_week); no cross-cut exclusion;
  * popularity = distinct clients in train;
  * co-purchase = train baskets containing both items;
  * ties: popularity, then item_id;
  * short list: missing positions are misses (no fill).
"""

from __future__ import annotations

import argparse
import collections
import itertools
import json
from pathlib import Path

import pandas as pd

TRAIN_END = "2026-08-31"
TEST_START = "2026-09-07"


def load(path: Path) -> tuple[pd.DataFrame, pd.DataFrame]:
    sales = pd.read_excel(path, sheet_name="Продажи", dtype=str)
    clients = pd.read_excel(path, sheet_name="Клиенты", dtype=str)[
        ["client_id", "client_group"]
    ]
    sales = sales.drop_duplicates(["order_id", "order_week", "item_id"])
    if "product_group" not in sales.columns:
        sales["product_group"] = ""
    sales["product_group"] = sales["product_group"].fillna("").astype(str)
    sales["product_group"] = sales.apply(
        lambda r: r["product_group"] or r["item_id"], axis=1
    )
    sales = sales.merge(clients, on="client_id", how="left")
    return sales, clients


def split(sales: pd.DataFrame):
    train = sales[sales["order_week"] <= TRAIN_END].copy()
    test = sales[sales["order_week"] >= TEST_START].copy()
    train_items = train.groupby("client_id")["item_id"].apply(set)
    test_new = {
        cid: set(group["item_id"]) - train_items.get(cid, set())
        for cid, group in test.groupby("client_id")
    }
    group_a = {cid for cid, new in test_new.items() if cid in train_items.index and new}
    group_b = {cid for cid in group_a if len(train_items[cid]) == 1}
    return train, test, train_items, test_new, group_a, group_b


class Baseline:
    def __init__(self, train: pd.DataFrame):
        self.train_items: dict[str, set[str]] = {}
        self.popularity = train.groupby("item_id")["client_id"].nunique().sort_values(
            ascending=False
        )
        self.item_pop = self.popularity.to_dict()

        self.pairs: collections.Counter[tuple[str, str]] = collections.Counter()
        for _, group in train.groupby(["order_id", "order_week"]):
            items = sorted(set(group["item_id"]))
            for a, b in itertools.combinations(items, 2):
                self.pairs[(a, b)] += 1

        self.neighbors: dict[str, list[tuple[str, int]]] = collections.defaultdict(list)
        for (a, b), count in self.pairs.items():
            self.neighbors[a].append((b, count))
            self.neighbors[b].append((a, count))

    def popularity_recs(self, owned: set[str], k: int = 3) -> list[str]:
        out = []
        for item in self.popularity.index:
            if item in owned:
                continue
            out.append(item)
            if len(out) == k:
                break
        return out

    def copurchase_recs(self, owned: set[str], k: int = 3) -> list[str]:
        scores: collections.defaultdict[str, int] = collections.defaultdict(int)
        for item in owned:
            for neighbor, count in self.neighbors.get(item, []):
                if neighbor not in owned:
                    scores[neighbor] += count
        ranked = sorted(
            scores.items(),
            key=lambda x: (-x[1], -self.item_pop.get(x[0], 0), x[0]),
        )
        return [item for item, _ in ranked[:k]]


def evaluate(model: Baseline, method: str, ids: set[str], train_items, test_new,
             item_group: dict[str, str], k=3):
    values = []
    filled = 0
    variant_hits, new_hits = 0, 0
    for cid in sorted(ids):
        owned = train_items.get(cid, set())
        rec = (
            model.popularity_recs(owned, k)
            if method == "popularity"
            else model.copurchase_recs(owned, k)
        )
        if rec:
            filled += 1
        hits = set(rec) & test_new.get(cid, set())
        values.append(len(hits) / k)
        known_groups = {item_group.get(i, i) for i in owned}
        for item in hits:
            if item_group.get(item, item) in known_groups:
                variant_hits += 1
            else:
                new_hits += 1
    if not values:
        return (0, 0, 0, 0, 0)
    return (sum(values) / len(values), filled / len(ids), len(ids), variant_hits, new_hits)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dataset", required=True, type=Path)
    ap.add_argument("--json", type=Path)
    args = ap.parse_args()

    sales, clients = load(args.dataset)
    train, test, train_items, test_new, group_a, group_b = split(sales)
    model = Baseline(train)
    model.train_items = train_items
    item_group = dict(zip(sales["item_id"], sales["product_group"]))

    print("Dataset:", args.dataset)
    print(
        f"clients={len(clients)} rows={len(sales)} "
        f"train_orders={train.order_id.nunique()} test_orders={test.order_id.nunique()} "
        f"A={len(group_a)} B={len(group_b)}"
    )

    rows = []
    for method in ["popularity", "copurchase"]:
        for group_name, group_ids in [("A", group_a), ("B", group_b)]:
            for cg in ["all", "B2B", "B2C"]:
                if cg == "all":
                    ids = group_ids
                else:
                    ids = group_ids & set(clients[clients.client_group == cg].client_id)
                p3, fill, n, variant_hits, new_hits = evaluate(
                    model, method, ids, train_items, test_new, item_group
                )
                rows.append(
                    {
                        "method": method,
                        "group": group_name,
                        "client_group": cg,
                        "n": n,
                        "precision_at_3": p3,
                        "fill_rate": fill,
                        "variant_hits": variant_hits,
                        "new_hits": new_hits,
                    }
                )
                hits = variant_hits + new_hits
                print(
                    f"{method:11s} {group_name} {cg:3s}: n={n:4d} "
                    f"P@3={p3*100:5.2f}% Fill={fill*100:5.1f}% "
                    f"variants={variant_hits} new={new_hits}"
                    + (f" (variant share={variant_hits/hits:.1%})" if hits else "")
                )

    if args.json:
        args.json.write_text(json.dumps(rows, ensure_ascii=False, indent=2), encoding="utf-8")
        print("Saved:", args.json)


if __name__ == "__main__":
    main()
