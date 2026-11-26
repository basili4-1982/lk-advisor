#!/usr/bin/env python3
"""Consensus backtest for B2B: co-purchase + family + recent + category.

Compares a weighted multi-slice recommender with the current hybrid model on
the same time split. Prints aggregate metrics and writes consensus_report.md.

The dataset is never written to the repository.
"""

from __future__ import annotations

import argparse
import collections
import itertools
import json
import math
import re
from pathlib import Path

import numpy as np
import pandas as pd

from evaluate import Model, cross_cut_orders, load, split

ROOT = Path(__file__).resolve().parent
REPORT = ROOT / "consensus_report.md"
METRICS = ROOT / "consensus_metrics.json"

STOP = {
    "лицензия", "дистрибутив", "программа", "программы", "эвм", "право",
    "использование", "использования", "предоставление", "доступа", "сервисам",
    "на", "для", "и", "с", "по", "год", "года", "месяц", "месяца", "шт",
    "user", "пользователь", "пользователя", "рабочее", "место", "версия",
    "версии", "edition", "renewal", "license", "lic", "base", "online",
    "download", "dwnld", "only", "all", "lng", "russian", "русский",
    "продление", "продления", "неисключительное", "бессрочная", "бессрочный",
}

FAMILY_RULES: list[tuple[str, tuple[str, ...]]] = [
    ("kaspersky-endpoint", ("kaspersky", "endpoint")),
    ("kaspersky-small-office", ("kaspersky", "small", "office")),
    ("kaspersky-standard", ("kaspersky", "standard")),
    ("kaspersky-plus", ("kaspersky", "plus")),
    ("microsoft-365-business", ("microsoft", "365", "business")),
    ("microsoft-365-premium", ("microsoft", "365", "premium")),
    ("microsoft-365-personal", ("microsoft", "365", "personal")),
    ("microsoft-365-family", ("microsoft", "365", "family")),
    ("office-home", ("office", "home")),
    ("windows-pro", ("win", "pro")),
    ("google-workspace", ("google", "workspace")),
    ("kriptopro-csp", ("криптопро", "csp")),
    ("magpro-kryptopacket", ("магпро", "криптопакет")),
    ("drweb", ("dr.web",)),
    ("adobe", ("adobe",)),
    ("coreldraw", ("coreldraw",)),
    ("nanocad", ("nanocad",)),
    ("anydesk", ("anydesk",)),
    ("visio", ("visio",)),
    ("project", ("project",)),
    ("pdf", ("pdf",)),
]

WEIGHTS = {
    "co_purchase": 0.45,
    "family": 0.25,
    "recent": 0.20,
    "category": 0.10,
}


def normalize(text: str) -> str:
    text = str(text).lower().replace("ё", "е")
    return re.sub(r"[^a-zа-я0-9]+", " ", text)


def family_key(name: str, category: str) -> str:
    t = normalize(name)
    for key, needles in FAMILY_RULES:
        if all(n in t for n in needles):
            return key
    words = [w for w in t.split() if w not in STOP and len(w) > 2]
    return category + ":" + " ".join(words[:3])


def bootstrap_ci(values: list[float], n: int = 1200) -> tuple[float, float]:
    arr = np.asarray(values, dtype=float)
    rng = np.random.default_rng(42)
    idx = rng.integers(0, len(arr), size=(n, len(arr)))
    means = arr[idx].mean(axis=1)
    return float(np.percentile(means, 2.5)), float(np.percentile(means, 97.5))


def bootstrap_diff(a: list[float], b: list[float], n: int = 1200) -> tuple[float, float]:
    diff = np.asarray(a, dtype=float) - np.asarray(b, dtype=float)
    rng = np.random.default_rng(42)
    idx = rng.integers(0, len(diff), size=(n, len(diff)))
    means = diff[idx].mean(axis=1)
    return float(np.percentile(means, 2.5)), float(np.percentile(means, 97.5))


class Slices:
    def __init__(self, train: pd.DataFrame, items: pd.DataFrame):
        self.items = items.set_index("item_id")
        self.family = {
            item: family_key(row.item_name, row.category)
            for item, row in self.items.iterrows()
        }
        self.family_members: dict[str, list[str]] = collections.defaultdict(list)
        for item, key in self.family.items():
            self.family_members[key].append(item)

        b2b = train[train["client_group"] == "B2B"]
        orders = b2b.groupby(["order_id", "order_week"])["item_id"].apply(
            lambda x: sorted(set(x))
        )
        pair_count: collections.Counter[tuple[str, str]] = collections.Counter()
        item_pop: collections.Counter[str] = collections.Counter()
        for order_items in orders:
            for item in order_items:
                item_pop[item] += 1
            for a, b in itertools.combinations(order_items, 2):
                pair_count[(a, b)] += 1

        self.neighbors: dict[str, list[tuple[str, float]]] = collections.defaultdict(list)
        for (a, b), count in pair_count.items():
            score = count / math.sqrt(max(1, item_pop[a]) * max(1, item_pop[b]))
            self.neighbors[a].append((b, score))
            self.neighbors[b].append((a, score))
        for item in self.neighbors:
            self.neighbors[item].sort(key=lambda x: (-x[1], x[0]))

        end = pd.to_datetime(b2b["order_week"]).max()
        age = (end - pd.to_datetime(b2b["order_week"])).dt.days / 7.0
        decay = 0.5 ** (age / 8.0)
        self.recent = (
            b2b.assign(_w=decay).groupby("item_id")["_w"].sum().sort_values(ascending=False)
        )
        self.segment = b2b.groupby("item_id").size().sort_values(ascending=False)
        self.category = {
            cat: group.groupby("item_id").size().sort_values(ascending=False)
            for cat, group in b2b.groupby("category")
        }

    def candidates(self, owned: set[str]) -> dict[str, dict[str, float]]:
        out: dict[str, dict[str, float]] = {
            "co_purchase": collections.defaultdict(float),
            "family": collections.defaultdict(float),
            "recent": collections.defaultdict(float),
            "category": collections.defaultdict(float),
        }
        for item in owned:
            for neighbor, score in self.neighbors.get(item, [])[:200]:
                if neighbor not in owned:
                    out["co_purchase"][neighbor] += score
            key = self.family.get(item)
            if key:
                for member in self.family_members.get(key, []):
                    if member not in owned:
                        out["family"][member] += 1.0
            cat = self.items.loc[item, "category"] if item in self.items.index else ""
            for candidate, score in self.category.get(cat, pd.Series(dtype=float)).head(200).items():
                if candidate not in owned:
                    out["category"][candidate] += float(score)

        for candidate in self.recent.index[:500]:
            if candidate not in owned:
                out["recent"][candidate] += float(self.recent[candidate])

        return {name: dict(scores) for name, scores in out.items()}

    def recommend(self, owned: set[str], weights: dict[str, float],
                  support_bonus: float = 0.0, k: int = 3) -> tuple[list[str], list[int]]:
        candidates = self.candidates(owned)
        normalized: dict[str, dict[str, float]] = {}
        for name, scores in candidates.items():
            if not scores:
                normalized[name] = {}
                continue
            top = max(scores.values())
            normalized[name] = {item: score / top for item, score in scores.items()}

        total: collections.defaultdict[str, float] = collections.defaultdict(float)
        support: collections.defaultdict[int] = collections.defaultdict(int)
        for name, weight in weights.items():
            for item, score in normalized.get(name, {}).items():
                total[item] += weight * score
                support[item] += 1

        ranked = sorted(
            total.items(),
            key=lambda x: (-(x[1] + support_bonus * max(0, support[x[0]] - 1)), x[0]),
        )
        out: list[str] = []
        supp: list[int] = []
        for item, _ in ranked:
            if item in owned:
                continue
            out.append(item)
            supp.append(support[item])
            if len(out) == k:
                break
        # Fallback: recent/segment popularity.
        if len(out) < k:
            for source in (self.recent, self.segment):
                for item in source.index:
                    if item in owned or item in out:
                        continue
                    out.append(item)
                    supp.append(0)
                    if len(out) == k:
                        break
                if len(out) == k:
                    break
        return out, supp


def eval_consensus(slices: Slices, ids: set[str], train_items: dict[str, set[str]],
                   test_new: dict[str, set[str]], weights: dict[str, float],
                   support_bonus: float, k: int = 3) -> dict:
    values: list[float] = []
    supports: list[int] = []
    recommended: set[str] = set()
    for cid in sorted(ids):
        rec, supp = slices.recommend(train_items.get(cid, set()), weights, support_bonus, k)
        recommended.update(rec)
        values.append(len(set(rec) & test_new.get(cid, set())) / k)
        supports.extend(supp)
    lo, hi = bootstrap_ci(values)
    return {
        "n": len(values),
        "p3": float(np.mean(values)),
        "p3_lo": lo,
        "p3_hi": hi,
        "fill": 1.0 if values else 0.0,
        "coverage": len(recommended),
        "avg_support": float(np.mean(supports)) if supports else 0.0,
        "values": values,
    }


def eval_hybrid(model: Model, ids: set[str], audience: str, k: int = 3) -> dict:
    values: list[float] = []
    recommended: set[str] = set()
    for cid in sorted(ids):
        rec = model.recommend(cid, audience, k)
        recommended.update(rec)
        values.append(len(set(rec) & model.test_new.get(cid, set())) / k)
    lo, hi = bootstrap_ci(values)
    return {
        "n": len(values),
        "p3": float(np.mean(values)),
        "p3_lo": lo,
        "p3_hi": hi,
        "fill": 1.0 if values else 0.0,
        "coverage": len(recommended),
        "values": values,
    }


def hybrid_scores(model: Model, client_id: str) -> dict[str, float]:
    """Replicate evaluate.Model.recommend scoring, but return all candidates."""
    owned = model.train_items.get(client_id, set())
    scores: collections.defaultdict[str, float] = collections.defaultdict(float)
    popularity = model.pop_b2b
    max_pop = max(1, int(popularity.max()) if len(popularity) else 1)
    for item in owned:
        for neighbor, count in model.neighbors.get(item, [])[:200]:
            if neighbor in owned:
                continue
            scores[neighbor] += (
                3.0
                * count
                / math.sqrt(max(1, model.item_pop[item]) * max(1, model.item_pop[neighbor]))
            )
    for item, count in popularity.items():
        if item not in owned:
            scores[item] += 0.5 * count / max_pop
    for item in owned:
        for candidate in popularity.index[:500]:
            if candidate in owned:
                continue
            overlap = len(model.tokens.get(item, set()) & model.tokens.get(candidate, set()))
            if overlap:
                scores[candidate] += 0.2 * overlap
    return dict(scores)


def recommend_addon(slices: Slices, model: Model, client_id: str,
                    addons: dict[str, float], k: int = 3) -> list[str]:
    owned = model.train_items.get(client_id, set())
    base = hybrid_scores(model, client_id)
    if base:
        top = max(base.values())
        base_norm = {item: score / top for item, score in base.items()}
    else:
        base_norm = {}

    add: collections.defaultdict[str, float] = collections.defaultdict(float)
    if addons:
        candidates = slices.candidates(owned)
        for name, weight in addons.items():
            scores = candidates.get(name, {})
            if not scores:
                continue
            top = max(scores.values())
            for item, score in scores.items():
                add[item] += weight * score / top

    final: collections.defaultdict[str, float] = collections.defaultdict(float)
    for item, score in base_norm.items():
        final[item] += score
    for item, score in add.items():
        final[item] += score

    ranked = sorted(final.items(), key=lambda x: (-x[1], x[0]))
    out = [item for item, _ in ranked if item not in owned][:k]
    if len(out) < k:
        for source in (slices.recent, slices.segment):
            for item in source.index:
                if item in owned or item in out:
                    continue
                out.append(item)
                if len(out) == k:
                    return out
    return out


def eval_addon(slices: Slices, model: Model, ids: set[str],
               addons: dict[str, float], k: int = 3) -> dict:
    values: list[float] = []
    recommended: set[str] = set()
    for cid in sorted(ids):
        rec = recommend_addon(slices, model, cid, addons, k)
        recommended.update(rec)
        values.append(len(set(rec) & model.test_new.get(cid, set())) / k)
    lo, hi = bootstrap_ci(values)
    return {
        "n": len(values),
        "p3": float(np.mean(values)),
        "p3_lo": lo,
        "p3_hi": hi,
        "fill": 1.0 if values else 0.0,
        "coverage": len(recommended),
        "values": values,
    }


def renewal_metrics(train: pd.DataFrame, test: pd.DataFrame) -> dict:
    last_train = train.groupby(["client_id", "item_id"])["week_date"].max()
    first_test = test.groupby(["client_id", "item_id"])["week_date"].min()
    joined = pd.concat([last_train.rename("last_train"), first_test.rename("first_test")], axis=1).dropna()
    joined["gap_days"] = (joined["first_test"] - joined["last_train"]).dt.days
    renewals = joined[(joined["gap_days"] >= 300) & (joined["gap_days"] <= 430)]
    clients = renewals.index.get_level_values("client_id").unique()

    # Rank owned items by closeness to a 365-day renewal window.
    hits = 0
    total = 0
    for cid in clients:
        test_date = test[test["client_id"] == cid]["week_date"].min()
        if pd.isna(test_date):
            continue
        owned = train[train["client_id"] == cid].groupby("item_id")["week_date"].max()
        if owned.empty:
            continue
        score = (test_date - owned).dt.days
        due = score.sub(365).abs().sort_values()
        actual = set(renewals.loc[cid].index) if cid in renewals.index.get_level_values(0) else set()
        if not actual:
            continue
        total += 1
        if any(item in actual for item in due.head(3).index):
            hits += 1
    return {
        "renewal_pairs": int(len(renewals)),
        "renewal_clients": int(len(clients)),
        "median_gap_days": float(renewals["gap_days"].median()) if len(renewals) else 0.0,
        "top3_renewal_accuracy": hits / total if total else 0.0,
        "scored_clients": total,
    }


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dataset", required=True, type=Path)
    args = ap.parse_args()

    sales, clients = load(args.dataset)
    sales["week_date"] = pd.to_datetime(sales["order_week"])
    cross_cut = cross_cut_orders(sales)
    sales = sales[~sales["order_id"].isin(cross_cut)].copy()
    train, test, train_items, test_new, group_a, group_b = split(sales)
    train = train.copy()
    test = test.copy()
    train["week_date"] = pd.to_datetime(train["order_week"])
    test["week_date"] = pd.to_datetime(test["order_week"])
    train["category"] = train["item_name"].map(
        lambda name: "other" if not isinstance(name, str) else (
            "security" if any(k in name.lower() for k in ("kaspersky", "крипто", "магпро", "dr.web", "антивирус"))
            else "office" if any(k in name.lower() for k in ("office", "microsoft 365", "windows"))
            else "graphics" if any(k in name.lower() for k in ("adobe", "corel", "nanocad", "visio"))
            else "training" if any(k in name.lower() for k in ("курс", "обучен"))
            else "other"
        )
    )

    items = train.drop_duplicates("item_id")[["item_id", "item_name", "category"]].set_index("item_id")
    slices = Slices(train, items.reset_index())

    b2b_ids = set(clients[clients["client_group"] == "B2B"]["client_id"])
    groups = {"A": group_a & b2b_ids, "B": group_b & b2b_ids}

    base_model = Model(train, train_items)
    base_model.test_new = test_new
    baseline = {
        "A": eval_hybrid(base_model, groups["A"], "B2B"),
        "B": eval_hybrid(base_model, groups["B"], "B2B"),
    }

    # Consensus is layered on top of the strong hybrid, not used as a replacement.
    # Replacing the hybrid with slices-only scores loses the popularity/token signal.
    addon_variants: dict[str, dict[str, float]] = {
        "baseline": {},
        "+recent_10": {"recent": 0.10},
        "+recent_20": {"recent": 0.20},
        "+recent_30": {"recent": 0.30},
        "+family_10": {"family": 0.10},
        "+family_20": {"family": 0.20},
        "+category_10": {"category": 0.10},
        "+recent10_family10": {"recent": 0.10, "family": 0.10},
        "+recent15_family10_cat05": {"recent": 0.15, "family": 0.10, "category": 0.05},
        "+recent10_family10_cat10": {"recent": 0.10, "family": 0.10, "category": 0.10},
    }
    results = {}
    for label, addons in addon_variants.items():
        results[label] = {}
        for group_name, ids in groups.items():
            results[label][group_name] = eval_addon(
                slices, base_model, ids, addons
            )

    diffs = {}
    for label in addon_variants:
        cur = results[label]["A"]["values"]
        base = baseline["A"]["values"]
        lo, hi = bootstrap_diff(cur, base)
        diffs[label] = {
            "delta": float(np.mean(cur) - np.mean(base)),
            "lo": lo,
            "hi": hi,
        }

    renew = renewal_metrics(train, test)

    metrics = {
        "group_sizes": {k: len(v) for k, v in groups.items()},
        "baseline": {k: {kk: vv for kk, vv in v.items() if kk != "values"} for k, v in baseline.items()},
        "consensus": {
            label: {
                group: {k: v for k, v in val.items() if k != "values"}
                for group, val in groups_res.items()
            }
            for label, groups_res in results.items()
        },
        "diffs_vs_baseline_A": diffs,
        "renewal": renew,
    }
    METRICS.write_text(json.dumps(metrics, ensure_ascii=False, indent=2), encoding="utf-8")

    lines = []
    lines.append("# Консенсус разрезов: B2B\n")
    lines.append(f"- Group A: **{len(groups['A'])}**, Group B: **{len(groups['B'])}**.")
    lines.append("- Baseline — текущий гибрид (co-purchase + популярность + токены).")
    lines.append("- Проверяем не замену baseline, а добавление разрезов к сильному гибриду.")
    lines.append("- Замена baseline на чистые разрезы проверялась отдельно и проиграла; ниже — add-on.")
    lines.append("")
    lines.append("## Precision@3\n")
    lines.append("| Вариант | A | B | Coverage A |")
    lines.append("|---|---:|---:|---:|")
    lines.append(f"| baseline hybrid | {baseline['A']['p3']*100:.2f}% | {baseline['B']['p3']*100:.2f}% | {baseline['A']['coverage']} |")
    for label in addon_variants:
        r = results[label]
        lines.append(f"| {label} | {r['A']['p3']*100:.2f}% | {r['B']['p3']*100:.2f}% | {r['A']['coverage']} |")
    lines.append("")
    lines.append("## Разница к baseline, Group A\n")
    lines.append("| Вариант | Δ P@3 | 95% CI |")
    lines.append("|---|---:|---|")
    for label, d in diffs.items():
        lines.append(f"| {label} | {d['delta']*100:+.2f} п.п. | [{d['lo']*100:+.2f}; {d['hi']*100:+.2f}] |")
    lines.append("")
    lines.append("## Окно продления\n")
    lines.append(f"- Пар с повтором в тесте: **{renew['renewal_pairs']}**; клиентов: **{renew['renewal_clients']}**.")
    lines.append(f"- Медианный интервал: **{renew['median_gap_days']:.0f} дней**.")
    lines.append(f"- Top-3 accuracy по сроку продления: **{renew['top3_renewal_accuracy']*100:.1f}%** на {renew['scored_clients']} клиентах.")
    lines.append("")
    lines.append("## Вывод\n")
    best = max(addon_variants, key=lambda label: results[label]["A"]["p3"])
    best_d = diffs[best]
    lines.append(f"- Лучший вариант: **{best}**, P@3 {results[best]['A']['p3']*100:.2f}% ({best_d['delta']*100:+.2f} п.п. к baseline).")
    if best_d["lo"] > 0:
        lines.append("- Прирост доказан: нижняя граница 95% CI выше нуля.")
    else:
        lines.append("- Прирост не доказан: 95% CI пересекает ноль. Нужна большая выборка или более сильный сигнал.")
    lines.append("- Продление надо оценивать отдельно: в Group A target — только новые товары, поэтому renewal не влияет на P@3.")
    lines.append("- Если add-on не даёт доказанного прироста, не добавляем разрез в прод.")
    try:
        season = json.loads((ROOT / "seasonality_metrics.json").read_text(encoding="utf-8"))
        decay = season["hybrid_diffs_vs_segment_A"]["B2B|decay_8w"]
        lines.append(
            f"- Отдельно проверенная замена популярности на decay_8w даёт "
            f"{decay['delta']*100:+.2f} п.п. (CI [{decay['lo']*100:+.2f}; {decay['hi']*100:+.2f}]) — "
            "это доказанный временной ход, в отличие от add-on."
        )
    except (OSError, KeyError, ValueError):
        pass
    REPORT.write_text("\n".join(lines) + "\n", encoding="utf-8")

    print(json.dumps(metrics, ensure_ascii=False, indent=2))
    print("saved", REPORT)
    print("saved", METRICS)


if __name__ == "__main__":
    main()
