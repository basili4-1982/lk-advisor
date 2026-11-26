#!/usr/bin/env python3
"""Measure temporal signals in the LK v3 dataset.

Questions:
  1. Is there a stable monthly/category seasonality?
  2. Do recent purchases predict the next period better than all history?
  3. Does "same month last year" beat global popularity?
  4. Is there a B2B renewal window?

The script reads the original XLSX, prints aggregate metrics and writes
seasonality_report.md + charts. It never writes raw data to the repository.
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd

from evaluate import TEST_START, TRAIN_END, Model, cross_cut_orders, load, split
from import_dataset import category_for

ROOT = Path(__file__).resolve().parent
REPORT = ROOT / "seasonality_report.md"
METRICS = ROOT / "seasonality_metrics.json"

AUDIENCES = ["B2B", "B2C"]


def bootstrap_ci(values: list[float], n: int = 1200) -> tuple[float, float]:
    arr = np.asarray(values, dtype=float)
    if len(arr) == 0:
        return (0.0, 0.0)
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


def popularity(train: pd.DataFrame, audience: str | None = None,
               since: pd.Timestamp | None = None,
               months: set[tuple[int, int]] | None = None,
               half_life_weeks: float | None = None) -> pd.Series:
    df = train
    if audience:
        df = df[df["client_group"] == audience]
    if since is not None:
        df = df[df["week_date"] >= since]
    if months is not None:
        df = df[df["month"].isin([pd.Period(year=y, month=m, freq="M") for y, m in months])]
    if half_life_weeks:
        end = train["week_date"].max()
        age = (end - df["week_date"]).dt.days / 7.0
        weights = 0.5 ** (age / half_life_weeks)
        return df.assign(_w=weights).groupby("item_id")["_w"].sum().sort_values(ascending=False)
    return df.groupby("item_id").size().sort_values(ascending=False)


def normalized(series: pd.Series) -> pd.Series:
    total = float(series.sum())
    if total <= 0:
        return series
    return series / total


def blend(a: pd.Series, b: pd.Series, wa: float = 0.6, wb: float = 0.4) -> pd.Series:
    out = normalized(a).multiply(wa).add(normalized(b).multiply(wb), fill_value=0.0)
    return out.sort_values(ascending=False)


def recommend(pop: pd.Series, owned: set[str], k: int = 3) -> list[str]:
    out: list[str] = []
    for item in pop.index:
        if item in owned:
            continue
        out.append(item)
        if len(out) == k:
            break
    return out


def evaluate(pop: pd.Series, ids: set[str], train_items: dict[str, set[str]],
             test_new: dict[str, set[str]], k: int = 3) -> dict:
    values: list[float] = []
    filled = 0
    for cid in sorted(ids):
        rec = recommend(pop, train_items.get(cid, set()), k)
        if rec:
            filled += 1
        values.append(len(set(rec) & test_new.get(cid, set())) / k)
    if not values:
        return {"n": 0, "p3": 0.0, "fill": 0.0, "values": []}
    lo, hi = bootstrap_ci(values)
    return {
        "n": len(values),
        "p3": float(np.mean(values)),
        "p3_lo": lo,
        "p3_hi": hi,
        "fill": filled / len(values),
        "values": values,
    }


def evaluate_hybrid(model: Model, ids: set[str], audience: str, k: int = 3) -> dict:
    values: list[float] = []
    filled = 0
    for cid in sorted(ids):
        rec = model.recommend(cid, audience, k)
        if rec:
            filled += 1
        values.append(len(set(rec) & model.test_new.get(cid, set())) / k)
    if not values:
        return {"n": 0, "p3": 0.0, "fill": 0.0, "values": []}
    lo, hi = bootstrap_ci(values)
    return {
        "n": len(values),
        "p3": float(np.mean(values)),
        "p3_lo": lo,
        "p3_hi": hi,
        "fill": filled / len(values),
        "values": values,
    }


def fmt_pct(x: float) -> str:
    return f"{x * 100:.2f}%"


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dataset", required=True, type=Path)
    args = ap.parse_args()

    sales, clients = load(args.dataset)
    sales["week_date"] = pd.to_datetime(sales["order_week"])
    sales["month"] = sales["week_date"].dt.to_period("M")
    sales["category"] = sales["item_name"].map(category_for)

    cross_cut = cross_cut_orders(sales)
    excluded_rows = int(sales["order_id"].isin(cross_cut).sum())
    sales = sales[~sales["order_id"].isin(cross_cut)].copy()

    train, test, train_items, test_new, group_a, group_b = split(sales)
    train = train.copy()
    test = test.copy()
    train["week_date"] = pd.to_datetime(train["order_week"])
    train["month"] = train["week_date"].dt.to_period("M")
    test["week_date"] = pd.to_datetime(test["order_week"])
    test["month"] = test["week_date"].dt.to_period("M")

    test_months = sorted(set(test["month"]))
    seasonal_months = {(p.year - 1, p.month) for p in test_months}
    last_train_week = train["week_date"].max()
    recent_12 = last_train_week - pd.Timedelta(weeks=12)
    recent_4 = last_train_week - pd.Timedelta(weeks=4)

    # ---------- 1. Monthly demand ----------
    monthly = sales.groupby(["month", "client_group"]).size().unstack(fill_value=0)
    monthly.index = monthly.index.to_timestamp()
    overall = monthly.sum(axis=1)
    cv = float(overall.std(ddof=0) / overall.mean()) if overall.mean() else 0.0
    peak_month = overall.idxmax().strftime("%Y-%m")
    low_month = overall.idxmin().strftime("%Y-%m")

    # ---------- 2. Category seasonality ----------
    cat_month = sales.groupby(["month", "category"]).size().unstack(fill_value=0)
    cat_share = cat_month.div(cat_month.sum(axis=1), axis=0)
    cat_stats = []
    for cat in cat_share.columns:
        series = cat_share[cat].dropna()
        if series.sum() <= 0:
            continue
        mean = float(series.mean())
        if mean <= 0:
            continue
        cat_stats.append({
            "category": cat,
            "rows": int(cat_month[cat].sum()),
            "mean_share": mean,
            "cv": float(series.std(ddof=0) / mean),
            "peak_month": series.idxmax().strftime("%Y-%m"),
            "peak_share": float(series.max()),
            "amplitude": float(series.max() / series.min()) if series.min() > 0 else math.inf,
        })
    cat_stats.sort(key=lambda x: (-x["rows"], x["category"]))

    # ---------- 3. Year-over-year repeatability ----------
    yoy = {}
    for audience, subset in [("all", sales),
                             ("B2B", sales[sales["client_group"] == "B2B"]),
                             ("B2C", sales[sales["client_group"] == "B2C"])]:
        m = subset.groupby("month").size()
        common = [(pd.Period(year=y, month=mo, freq="M")) for y in (2025, 2026) for mo in range(1, 11)]
        a = m.reindex([p for p in common if p.year == 2025], fill_value=0)
        b = m.reindex([p for p in common if p.year == 2026], fill_value=0)
        yoy[audience] = float(np.corrcoef(a, b)[0, 1]) if a.std() and b.std() else 0.0

    # Item-level: items with enough rows in both Jan-Oct windows.
    item_yoy = []
    for item, group in sales.groupby("item_id"):
        m25 = group[(group["month"].dt.year == 2025) & (group["month"].dt.month <= 10)].groupby("month").size()
        m26 = group[(group["month"].dt.year == 2026) & (group["month"].dt.month <= 10)].groupby("month").size()
        if m25.sum() < 20 or m26.sum() < 20:
            continue
        m25 = m25.reindex([pd.Period(year=2025, month=mo, freq="M") for mo in range(1, 11)], fill_value=0)
        m26 = m26.reindex([pd.Period(year=2026, month=mo, freq="M") for mo in range(1, 11)], fill_value=0)
        if m25.std() == 0 or m26.std() == 0:
            continue
        item_yoy.append(float(np.corrcoef(m25, m26)[0, 1]))
    item_yoy_arr = np.asarray(item_yoy)

    # ---------- 4. B2B renewal intervals ----------
    renewals = []
    ordered = sales.sort_values(["client_id", "item_id", "week_date"])
    prev = ordered.groupby(["client_id", "item_id"])["week_date"].shift(1)
    gaps = (ordered["week_date"] - prev).dt.days.dropna()
    gaps = gaps[(gaps >= 0) & (gaps <= 900)]
    renewal_300_430 = gaps[(gaps >= 300) & (gaps <= 430)]
    repeat_pairs = int(ordered.groupby(["client_id", "item_id"]).size().gt(1).sum())

    # ---------- 5. Backtest popularity variants ----------
    global_pop = popularity(train)
    segment_pop = {a: popularity(train, audience=a) for a in AUDIENCES}
    recent12_pop = {a: popularity(train, audience=a, since=recent_12) for a in AUDIENCES}
    recent4_pop = {a: popularity(train, audience=a, since=recent_4) for a in AUDIENCES}
    seasonal_pop = {a: popularity(train, audience=a, months=seasonal_months) for a in AUDIENCES}
    decay_pop = {a: popularity(train, audience=a, half_life_weeks=8) for a in AUDIENCES}
    blend_pop = {a: blend(segment_pop[a], seasonal_pop[a]) for a in AUDIENCES}
    blend_recent_pop = {a: blend(segment_pop[a], recent12_pop[a], 0.5, 0.5) for a in AUDIENCES}

    # All-audience variants for the B2C part of the hybrid model.
    all_recent12 = popularity(train, since=recent_12)
    all_recent4 = popularity(train, since=recent_4)
    all_seasonal = popularity(train, months=seasonal_months)
    all_decay = popularity(train, half_life_weeks=8)
    all_blend = blend(global_pop, all_seasonal)

    variants: dict[str, dict[str, pd.Series]] = {
        "global_all": {a: global_pop for a in AUDIENCES},
        "segment": segment_pop,
        "recent_12w": recent12_pop,
        "recent_4w": recent4_pop,
        "seasonal_same_month": seasonal_pop,
        "decay_8w": decay_pop,
        "blend_60_40": blend_pop,
        "blend_recent_50_50": blend_recent_pop,
    }

    results: dict[str, dict[str, dict]] = {}
    for audience in AUDIENCES:
        ids = set(clients[clients["client_group"] == audience]["client_id"])
        results[audience] = {}
        for label, pops in variants.items():
            for group_name, group_ids in [("A", group_a), ("B", group_b)]:
                target = group_ids & ids
                key = f"{label}|{group_name}"
                results[audience][key] = evaluate(
                    pops[audience], target, train_items, test_new
                )

    # ---------- 5b. Hybrid + temporal ----------
    hybrid_variants: dict[str, tuple[pd.Series, pd.Series]] = {
        "segment": (segment_pop["B2B"], segment_pop["B2C"]),
        "global_all": (global_pop, global_pop),
        "recent_12w": (recent12_pop["B2B"], all_recent12),
        "recent_4w": (recent4_pop["B2B"], all_recent4),
        "seasonal_same_month": (seasonal_pop["B2B"], all_seasonal),
        "decay_8w": (decay_pop["B2B"], all_decay),
        "blend_60_40": (blend_pop["B2B"], all_blend),
    }
    hybrid_results: dict[str, dict[str, dict]] = {}
    for label, (pop_b2b, pop_all) in hybrid_variants.items():
        model = Model(train, train_items)
        model.pop_b2b = pop_b2b
        model.pop_all = pop_all
        model.test_new = test_new
        hybrid_results[label] = {}
        for audience in AUDIENCES:
            ids = set(clients[clients["client_group"] == audience]["client_id"])
            for group_name, group_ids in [("A", group_a), ("B", group_b)]:
                target = group_ids & ids
                hybrid_results[label][f"{audience}|{group_name}"] = evaluate_hybrid(
                    model, target, audience
                )

    hybrid_diffs = {}
    for audience in AUDIENCES:
        base = hybrid_results["segment"][f"{audience}|A"]["values"]
        for label in hybrid_variants:
            if label == "segment":
                continue
            cur = hybrid_results[label][f"{audience}|A"]["values"]
            if len(cur) != len(base):
                continue
            lo, hi = bootstrap_diff(cur, base)
            hybrid_diffs[f"{audience}|{label}"] = {
                "delta": float(np.mean(cur) - np.mean(base)),
                "lo": lo,
                "hi": hi,
            }

    # Paired differences vs segment popularity, Group A.
    diffs = {}
    for audience in AUDIENCES:
        base = results[audience]["segment|A"]["values"]
        for label in variants:
            if label == "segment":
                continue
            cur = results[audience][f"{label}|A"]["values"]
            if len(cur) != len(base):
                continue
            lo, hi = bootstrap_diff(cur, base)
            diffs[f"{audience}|{label}"] = {
                "delta": float(np.mean(cur) - np.mean(base)),
                "lo": lo,
                "hi": hi,
            }

    # ---------- 6. Seasonal lift potential ----------
    window = sales[sales["month"].isin([pd.Period(year=2025, month=m, freq="M") for m in (9, 10)])]
    baseline = sales[sales["month"].dt.year == 2025]
    lift_items = set()
    for item, group in window.groupby("item_id"):
        annual = baseline[baseline["item_id"] == item]
        if len(annual) < 10:
            continue
        expected = len(annual) * 2 / 12
        if expected > 0 and len(group) >= 1.5 * expected:
            lift_items.add(item)
    test_rows = test
    lift_share = float(test_rows["item_id"].isin(lift_items).mean()) if len(test_rows) else 0.0

    # ---------- 7. Charts ----------
    fig, ax = plt.subplots(figsize=(10, 4.5))
    for audience, color in [("B2B", "#1d4ed8"), ("B2C", "#c8102e")]:
        if audience in monthly.columns:
            ax.plot(monthly.index, monthly[audience], marker="o", label=audience, color=color)
    ax.set_title("Покупки по месяцам")
    ax.set_ylabel("строки продаж")
    ax.grid(alpha=0.25)
    ax.legend()
    fig.tight_layout()
    fig.savefig(ROOT / "seasonality_monthly.png", dpi=160)
    plt.close(fig)

    top_cats = [x["category"] for x in cat_stats[:6]]
    fig, ax = plt.subplots(figsize=(10, 4.5))
    for cat in top_cats:
        ax.plot(cat_share.index.to_timestamp(), cat_share[cat], label=cat)
    ax.set_title("Доля категорий по месяцам")
    ax.set_ylabel("доля месяца")
    ax.grid(alpha=0.25)
    ax.legend(ncol=3, fontsize=8)
    fig.tight_layout()
    fig.savefig(ROOT / "seasonality_categories.png", dpi=160)
    plt.close(fig)

    labels = list(variants.keys())
    x = np.arange(len(labels))
    fig, axes = plt.subplots(1, 2, figsize=(14, 4.5), sharey=True)
    for ax, audience in zip(axes, AUDIENCES):
        vals = [results[audience][f"{label}|A"]["p3"] * 100 for label in labels]
        lo = [results[audience][f"{label}|A"]["p3_lo"] * 100 for label in labels]
        hi = [results[audience][f"{label}|A"]["p3_hi"] * 100 for label in labels]
        ax.bar(x, vals, color="#1d4ed8" if audience == "B2B" else "#c8102e", alpha=0.85)
        ax.errorbar(x, vals, yerr=[np.array(vals) - np.array(lo), np.array(hi) - np.array(vals)],
                    fmt="none", ecolor="#111827", capsize=3, lw=1)
        ax.set_xticks(x, labels, rotation=35, ha="right", fontsize=8)
        ax.set_title(f"{audience}, Group A, Precision@3")
        ax.set_ylabel("%")
        ax.grid(axis="y", alpha=0.25)
    fig.tight_layout()
    fig.savefig(ROOT / "seasonality_backtest.png", dpi=160)
    plt.close(fig)

    # ---------- 8. Report ----------
    metrics = {
        "rows": int(len(sales)),
        "excluded_cross_cut_orders": len(cross_cut),
        "excluded_rows": excluded_rows,
        "weeks": int(sales["week_date"].nunique()),
        "first_week": str(sales["week_date"].min().date()),
        "last_week": str(sales["week_date"].max().date()),
        "test_months": [str(p) for p in test_months],
        "seasonal_months": sorted([f"{y}-{m:02d}" for y, m in seasonal_months]),
        "monthly_cv": cv,
        "peak_month": peak_month,
        "low_month": low_month,
        "yoy_correlation": yoy,
        "item_yoy_n": int(len(item_yoy_arr)),
        "item_yoy_median": float(np.median(item_yoy_arr)) if len(item_yoy_arr) else 0.0,
        "item_yoy_share_gt_05": float(np.mean(item_yoy_arr > 0.5)) if len(item_yoy_arr) else 0.0,
        "repeat_pairs": repeat_pairs,
        "renewal_gap_median_days": float(renewal_300_430.median()) if len(renewal_300_430) else 0.0,
        "renewal_share_300_430": float(len(renewal_300_430) / len(gaps)) if len(gaps) else 0.0,
        "seasonal_lift_items": len(lift_items),
        "seasonal_lift_test_share": lift_share,
        "backtest": {
            audience: {
                key: {k: v for k, v in val.items() if k != "values"}
                for key, val in results[audience].items()
            }
            for audience in AUDIENCES
        },
        "diffs_vs_segment_A": diffs,
        "hybrid_diffs_vs_segment_A": hybrid_diffs,
        "hybrid_backtest": {
            label: {
                key: {k: v for k, v in val.items() if k != "values"}
                for key, val in result.items()
            }
            for label, result in hybrid_results.items()
        },
        "category_seasonality": cat_stats[:12],
    }
    METRICS.write_text(json.dumps(metrics, ensure_ascii=False, indent=2), encoding="utf-8")

    lines = []
    lines.append("# Сезонность и время: что показывают данные\n")
    lines.append(f"- Период: **{metrics['first_week']} — {metrics['last_week']}**, {metrics['weeks']} недель.")
    lines.append(f"- Строк после исключения cross-cut: **{len(sales):,}** (исключено {excluded_rows} строк / {len(cross_cut)} заказов).")
    lines.append(f"- Тестовые месяцы: **{', '.join(metrics['test_months'])}**; сезонные месяцы: **{', '.join(metrics['seasonal_months'])}**.")
    lines.append("")
    lines.append("## 1. Общая сезонность\n")
    lines.append(f"- CV месячного спроса: **{cv:.3f}**.")
    lines.append(f"- Пик: **{peak_month}**, минимум: **{low_month}**.")
    lines.append(f"- Корреляция месячного спроса 2025 vs 2026 (янв–окт): **{yoy['all']:.2f}**; B2B **{yoy['B2B']:.2f}**; B2C **{yoy['B2C']:.2f}**.")
    lines.append("")
    lines.append("## 2. Сезонность категорий\n")
    lines.append("| Категория | Строк | CV | Пик | Доля пика | Амплитуда |")
    lines.append("|---|---:|---:|---|---:|---:|")
    for c in cat_stats[:8]:
        amp = "—" if math.isinf(c["amplitude"]) else f"{c['amplitude']:.1f}x"
        lines.append(f"| {c['category']} | {c['rows']:,} | {c['cv']:.2f} | {c['peak_month']} | {c['peak_share']*100:.1f}% | {amp} |")
    lines.append("")
    lines.append("## 3. Повторяемость на уровне товара\n")
    lines.append(f"- Товаров с достаточным объёмом в обоих годах: **{len(item_yoy_arr)}**.")
    lines.append(f"- Медиана корреляции месячных профилей: **{metrics['item_yoy_median']:.2f}**.")
    lines.append(f"- Доля товаров с корреляцией > 0.5: **{metrics['item_yoy_share_gt_05']*100:.1f}%**.")
    lines.append("")
    lines.append("## 4. Окно продления B2B\n")
    lines.append(f"- Пар клиент–товар с повторной покупкой: **{repeat_pairs:,}**.")
    lines.append(f"- Медианный интервал в окне 300–430 дней: **{metrics['renewal_gap_median_days']:.0f} дней**.")
    lines.append(f"- Доля повторов, попавших в 300–430 дней: **{metrics['renewal_share_300_430']*100:.1f}%**.")
    lines.append("")
    lines.append("## 5. Бэктест: даёт ли «в это время» прирост\n")
    lines.append("Precision@3 на Group A, 95% bootstrap CI. База — популярность сегмента.\n")
    lines.append("| Вариант | B2B A | B2B B | B2C A | B2C B |")
    lines.append("|---|---:|---:|---:|---:|")
    for label in variants:
        row = [label]
        for audience, group in [("B2B", "A"), ("B2B", "B"), ("B2C", "A"), ("B2C", "B")]:
            r = results[audience][f"{label}|{group}"]
            row.append(f"{r['p3']*100:.2f}% [{r['p3_lo']*100:.2f}; {r['p3_hi']*100:.2f}]")
        lines.append("| " + " | ".join(row) + " |")
    lines.append("")
    lines.append("Разница к популярности сегмента, Group A:\n")
    lines.append("| Аудитория | Вариант | Δ P@3 | 95% CI |")
    lines.append("|---|---|---:|---|")
    for key, d in diffs.items():
        audience, label = key.split("|", 1)
        lines.append(f"| {audience} | {label} | {d['delta']*100:+.2f} п.п. | [{d['lo']*100:+.2f}; {d['hi']*100:+.2f}] |")
    lines.append("")
    lines.append("## 6. Гибридная модель + время\n")
    lines.append("Тот же гибрид (co-purchase + популярность + токены), но популярность заменена временным вариантом. Precision@3, Group A/B.\n")
    lines.append("| Вариант популярности | B2B A | B2B B | B2C A | B2C B |")
    lines.append("|---|---:|---:|---:|---:|")
    for label in hybrid_variants:
        row = [label]
        for audience, group in [("B2B", "A"), ("B2B", "B"), ("B2C", "A"), ("B2C", "B")]:
            r = hybrid_results[label][f"{audience}|{group}"]
            row.append(f"{r['p3']*100:.2f}%")
        lines.append("| " + " | ".join(row) + " |")
    lines.append("")
    lines.append("Разница к гибриду с сегментной популярностью, Group A:\n")
    lines.append("| Аудитория | Вариант | Δ P@3 | 95% CI |")
    lines.append("|---|---|---:|---|")
    for key, d in hybrid_diffs.items():
        audience, label = key.split("|", 1)
        lines.append(f"| {audience} | {label} | {d['delta']*100:+.2f} п.п. | [{d['lo']*100:+.2f}; {d['hi']*100:+.2f}] |")
    lines.append("")
    lines.append("## 7. Потенциал сезонного лифта\n")
    lines.append(f"- Товаров с лифтом ≥1.5x в сентябре–октябре 2025: **{len(lift_items)}**.")
    lines.append(f"- Их доля в тестовых покупках: **{lift_share*100:.1f}%**.")
    lines.append("")
    lines.append("## Вывод\n")
    # Deterministic conclusion based on the measured deltas.
    b2b_best = max(
        ((label, diffs.get(f"B2B|{label}", {}).get("delta", 0.0)) for label in variants if label != "segment"),
        key=lambda x: x[1],
    )
    b2c_best = max(
        ((label, diffs.get(f"B2C|{label}", {}).get("delta", 0.0)) for label in variants if label != "segment"),
        key=lambda x: x[1],
    )
    lines.append(f"- Лучший временной вариант для B2B относительно сегментной популярности: **{b2b_best[0]}** ({b2b_best[1]*100:+.2f} п.п.).")
    lines.append(f"- Лучший временной вариант для B2C: **{b2c_best[0]}** ({b2c_best[1]*100:+.2f} п.п.).")
    hybrid_b2b = hybrid_results["segment"]["B2B|A"]["p3"]
    hybrid_best_label, hybrid_best_p3 = max(
        ((label, hybrid_results[label]["B2B|A"]["p3"]) for label in hybrid_variants),
        key=lambda x: x[1],
    )
    lines.append(f"- В гибриде B2B Group A: базовая сегментная популярность {hybrid_b2b*100:.2f}%, лучший вариант {hybrid_best_label} {hybrid_best_p3*100:.2f}%.")
    for key, d in hybrid_diffs.items():
        if key.startswith("B2B|") and d["lo"] > 0:
            lines.append(f"- **{key.split('|',1)[1]}** даёт доказанный прирост в гибриде B2B: {d['delta']*100:+.2f} п.п. (CI [{d['lo']*100:+.2f}; {d['hi']*100:+.2f}]).")
    lines.append("- Если CI разницы пересекает ноль — сигнал не доказан на этой выборке.")
    lines.append("- Сезонность по категориям считать можно; на уровне отдельного товара она шумная.")
    lines.append("- Для B2B отдельно смотреть окно продления, а не только календарный месяц.")
    REPORT.write_text("\n".join(lines) + "\n", encoding="utf-8")

    print(json.dumps({k: v for k, v in metrics.items() if k not in {"backtest", "category_seasonality"}}, ensure_ascii=False, indent=2))
    print("saved", REPORT)
    print("saved", METRICS)


if __name__ == "__main__":
    main()
