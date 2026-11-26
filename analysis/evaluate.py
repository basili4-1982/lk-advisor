#!/usr/bin/env python3
"""Evaluate recommendation models on the LK v3 dataset.

Usage:
    python3 analysis/evaluate.py --dataset "/path/to/Датасет_Рекомендации_ЛК_v3.xlsx"

The dataset is not stored in the repository. The script reads the original
XLSX and prints Precision@3 / Fill Rate for the protocol groups.
"""

from __future__ import annotations

import argparse
import collections
import itertools
import json
import math
from pathlib import Path

import pandas as pd

TRAIN_END = "2026-08-31"
TEST_START = "2026-09-07"


def load(path: Path) -> tuple[pd.DataFrame, pd.DataFrame]:
    sales = pd.read_excel(path, sheet_name="Продажи", dtype=str)
    clients = pd.read_excel(path, sheet_name="Клиенты", dtype=str)[
        ["client_id", "client_group", "industry", "segment", "lk_access"]
    ]
    sales = sales.merge(clients, on="client_id", how="left")
    return sales, clients


def split(sales: pd.DataFrame):
    train = sales[sales["order_week"] <= TRAIN_END]
    test = sales[sales["order_week"] >= TEST_START]
    train_items = train.groupby("client_id")["item_id"].apply(set)
    test_new = {
        cid: set(group["item_id"]) - train_items.get(cid, set())
        for cid, group in test.groupby("client_id")
    }
    group_a = {cid for cid, new in test_new.items() if cid in train_items.index and new}
    group_b = {cid for cid in group_a if len(train_items[cid]) == 1}
    return train, test, train_items, test_new, group_a, group_b


def tokens(name: str) -> set[str]:
    return set(
        str(name)
        .lower()
        .replace("ё", "е")
        .replace(",", " ")
        .replace(".", " ")
        .replace("-", " ")
        .replace("(", " ")
        .replace(")", " ")
        .split()
    )


class Model:
    def __init__(self, train: pd.DataFrame, train_items: dict[str, set[str]]):
        self.train_items = train_items

        # Popularity is counted like the organisers' baseline: over sale rows.
        self.pop_all = train.groupby("item_id").size().sort_values(ascending=False)
        self.pop_b2b = (
            train[train["client_group"] == "B2B"]
            .groupby("item_id")
            .size()
            .sort_values(ascending=False)
        )

        # Co-purchase pairs are built from order instances: (order_id, order_week).
        # The same order_id may span weeks, so grouping by order_id alone would merge them.
        orders = train.groupby(["order_id", "order_week"])["item_id"].apply(
            lambda x: sorted(set(x))
        )
        pairs: collections.Counter[tuple[str, str]] = collections.Counter()
        self.item_pop: collections.Counter[str] = collections.Counter()
        for items in orders:
            for item in items:
                self.item_pop[item] += 1
            if len(items) > 1:
                for a, b in itertools.combinations(items, 2):
                    pairs[(a, b)] += 1
                    pairs[(b, a)] += 1

        self.neighbors: dict[str, list[tuple[str, int]]] = collections.defaultdict(list)
        for (a, b), count in pairs.items():
            self.neighbors[a].append((b, count))
        for item in self.neighbors:
            self.neighbors[item].sort(key=lambda x: (-x[1], x[0]))

        names = train.drop_duplicates("item_id").set_index("item_id")["item_name"]
        self.tokens = {item: tokens(name) for item, name in names.items()}

    def recommend(self, client_id: str, group: str, k: int = 3) -> list[str]:
        owned = self.train_items.get(client_id, set())
        if group == "B2C":
            # B2C buys a narrow mass set; popularity beat every hybrid we tried.
            return [i for i in self.pop_all.index if i not in owned][:k]

        scores: collections.defaultdict[str, float] = collections.defaultdict(float)
        popularity = self.pop_b2b
        max_pop = max(1, popularity.max())

        for owned_item in owned:
            for neighbor, count in self.neighbors.get(owned_item, [])[:200]:
                if neighbor in owned:
                    continue
                scores[neighbor] += (
                    3.0
                    * count
                    / math.sqrt(max(1, self.item_pop[owned_item]) * max(1, self.item_pop[neighbor]))
                )
        for item, count in popularity.items():
            if item not in owned:
                scores[item] += 0.5 * count / max_pop
        for owned_item in owned:
            for item in popularity.index[:500]:
                if item in owned:
                    continue
                overlap = len(self.tokens.get(owned_item, set()) & self.tokens.get(item, set()))
                if overlap:
                    scores[item] += 0.2 * overlap

        ranked = sorted(scores.items(), key=lambda x: (-x[1], x[0]))
        out = [item for item, _ in ranked[:k]]
        if len(out) < k:
            out += [i for i in popularity.index if i not in owned and i not in out][: k - len(out)]
        return out


def evaluate(model: Model, ids: set[str], group: str) -> tuple[float, float, int]:
    values, filled = [], 0
    for client_id in ids:
        rec = model.recommend(client_id, group)
        if rec:
            filled += 1
        values.append(len(set(rec) & model.test_new[client_id]) / 3)
    return sum(values) / len(values), filled / len(ids), len(ids)


def cross_cut_orders(sales: pd.DataFrame) -> set[str]:
    weeks = sales.groupby("order_id")["order_week"].agg(["min", "max"])
    mask = (weeks["min"] <= TRAIN_END) & (weeks["max"] >= TEST_START)
    return set(weeks[mask].index)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", required=True, type=Path)
    parser.add_argument("--json", type=Path)
    args = parser.parse_args()

    sales, clients = load(args.dataset)
    cross_cut = cross_cut_orders(sales)
    excluded_rows = int(sales["order_id"].isin(cross_cut).sum())
    sales = sales[~sales["order_id"].isin(cross_cut)].copy()
    print(
        f"excluded cross-cut orders={len(cross_cut)} rows={excluded_rows} "
        f"(no future leakage into train/test)"
    )
    train, test, train_items, test_new, group_a, group_b = split(sales)
    model = Model(train, train_items)
    model.test_new = test_new

    print("Dataset:", args.dataset)
    train_instances = train[["order_id", "order_week"]].drop_duplicates().shape[0]
    test_instances = test[["order_id", "order_week"]].drop_duplicates().shape[0]
    print(
        f"clients={len(clients)} sales={len(sales)} "
        f"train_orders={train.order_id.nunique()} train_instances={train_instances} "
        f"test_orders={test.order_id.nunique()} test_instances={test_instances}"
    )
    print(f"Group A={len(group_a)}  Group B={len(group_b)}")

    rows = []
    for label, group in [("A", group_a), ("B", group_b)]:
        for client_group in ["B2B", "B2C"]:
            ids = group & set(clients[clients.client_group == client_group].client_id)
            precision, fill, n = evaluate(model, ids, client_group)
            rows.append(
                {
                    "group": label,
                    "client_group": client_group,
                    "n": n,
                    "precision_at_3": precision,
                    "fill_rate": fill,
                }
            )
            print(f"{label} {client_group}: n={n} P@3={precision:.4f} Fill={fill:.3f}")
        ids = group
        precision, fill, n = evaluate(model, ids, "B2B")
        # overall group: use B2B model for B2B clients and B2C popularity for B2C clients
        values, filled = [], 0
        for client_id in ids:
            client_group = clients.loc[clients.client_id == client_id, "client_group"].iloc[0]
            rec = model.recommend(client_id, client_group)
            if rec:
                filled += 1
            values.append(len(set(rec) & test_new[client_id]) / 3)
        print(f"{label} all: n={n} P@3={sum(values)/len(values):.4f} Fill={filled/n:.3f}")

    if args.json:
        args.json.write_text(json.dumps(rows, ensure_ascii=False, indent=2), encoding="utf-8")
        print("Saved:", args.json)


if __name__ == "__main__":
    main()
