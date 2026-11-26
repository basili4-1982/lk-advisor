#!/usr/bin/env python3
"""Toy simulation of the strategy bandit from cold start to a stable choice.

Three arms represent three product strategies:
  popular  -> product A
  usecase  -> product B
  training -> product C

The true click probabilities are invented. The bandit must discover that
"usecase" (product B) works best for this audience.
"""

from __future__ import annotations

import csv
import random
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd

ROUNDS = 220
MIN_OBSERVED = 20
EPSILON = 0.1
SEED = 42

ARMS = ["popular", "usecase", "training"]
PRODUCTS = {"popular": "A · antivirus", "usecase": "B · backup", "training": "C · security course"}
TRUE_CTR = {"popular": 0.08, "usecase": 0.65, "training": 0.20}
DEFAULT = "popular"


def score(shown: int, clicks: int, alpha: float, beta: float) -> float:
    return (clicks + alpha) / (shown + alpha + beta)


def choose(stats: dict[str, dict[str, float]], rng: random.Random) -> str:
    if stats[DEFAULT]["shown"] < MIN_OBSERVED:
        return DEFAULT
    under = [a for a in ARMS if a != DEFAULT and stats[a]["shown"] < MIN_OBSERVED]
    if under:
        return rng.choice(under)
    if rng.random() < EPSILON:
        return rng.choice(ARMS)
    best = DEFAULT
    best_score = score(stats[DEFAULT]["shown"], stats[DEFAULT]["clicks"],
                       stats[DEFAULT]["alpha"], stats[DEFAULT]["beta"])
    for arm in ARMS:
        if arm == DEFAULT or stats[arm]["shown"] < MIN_OBSERVED:
            continue
        s = score(stats[arm]["shown"], stats[arm]["clicks"],
                  stats[arm]["alpha"], stats[arm]["beta"])
        if s > best_score + 0.02:
            best, best_score = arm, s
    return best


def main() -> None:
    rng = random.Random(SEED)
    stats = {
        "popular": {"shown": 0, "clicks": 0, "alpha": 3, "beta": 2},
        "usecase": {"shown": 0, "clicks": 0, "alpha": 2, "beta": 3},
        "training": {"shown": 0, "clicks": 0, "alpha": 2, "beta": 3},
    }
    rows = []
    for round_no in range(1, ROUNDS + 1):
        arm = choose(stats, rng)
        clicked = 1 if rng.random() < TRUE_CTR[arm] else 0
        stats[arm]["shown"] += 1
        stats[arm]["clicks"] += clicked
        rows.append({
            "round": round_no,
            "arm": arm,
            "product": PRODUCTS[arm],
            "clicked": clicked,
        })

    df = pd.DataFrame(rows)
    df["arm_share_20"] = 0.0
    for arm in ARMS:
        df[f"is_{arm}"] = (df["arm"] == arm).astype(float)
        df[f"share_{arm}"] = df[f"is_{arm}"].rolling(20, min_periods=1).mean()
    out = Path(__file__).with_name("bandit_simulation.csv")
    df.to_csv(out, index=False)

    fig, axes = plt.subplots(2, 1, figsize=(12, 8), sharex=True)
    colors = {"popular": "#c8102e", "usecase": "#1d4ed8", "training": "#047857"}
    for arm in ARMS:
        axes[0].plot(df["round"], df[f"share_{arm}"], label=f"{arm} · {PRODUCTS[arm]}",
                     color=colors[arm], linewidth=2)
    axes[0].axvline(MIN_OBSERVED, color="#6b7280", linestyle="--", linewidth=1)
    axes[0].text(MIN_OBSERVED + 3, 0.92, "cold start ends", color="#6b7280", fontsize=9)
    axes[0].set_ylabel("share of traffic (rolling 20)")
    axes[0].set_ylim(0, 1)
    axes[0].set_title("Bandit: recommendation weights from cold start to stable choice")
    axes[0].legend(loc="center right", fontsize=9)
    axes[0].grid(alpha=0.25)

    for arm in ARMS:
        sub = df[df["arm"] == arm]
        if len(sub) == 0:
            continue
        axes[1].plot(sub["round"], sub["clicked"].expanding().mean(),
                     label=f"{arm} cumulative CTR", color=colors[arm], linewidth=2)
    axes[1].set_xlabel("session")
    axes[1].set_ylabel("cumulative CTR")
    axes[1].set_ylim(0, 1)
    axes[1].legend(loc="upper right", fontsize=9)
    axes[1].grid(alpha=0.25)

    fig.tight_layout()
    png = Path(__file__).with_name("bandit_simulation.png")
    fig.savefig(png, dpi=160)
    print("saved", png)
    print("saved", out)
    print("\nfinal stats:")
    for arm in ARMS:
        s = stats[arm]
        print(f"  {arm:8s} shown={s['shown']:3d} clicks={int(s['clicks']):3d} "
              f"ctr={s['clicks']/s['shown']:.3f} share={s['shown']/ROUNDS:.2f}")
    last = df.tail(20)
    print("\nlast 20 rounds:")
    print(last["arm"].value_counts(normalize=True).round(2).to_string())
    print("\nfirst 10 rounds:", df.head(10)["arm"].tolist())
    print("last 10 rounds: ", df.tail(10)["arm"].tolist())


if __name__ == "__main__":
    main()
