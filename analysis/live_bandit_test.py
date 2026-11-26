#!/usr/bin/env python3
"""Live API simulation: watch the real strategy bandit move from cold start.

It sends real /v1/recommendations requests for one B2C client and simulates
clicks with different probabilities for two strategies. The chart is built
from the strategy field returned by the service.
"""

from __future__ import annotations

import csv
import json
import random
import urllib.request
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd

BASE = "http://localhost:8080"
CLIENT = "lk-36612"
ROUNDS = 180
SEED = 7
TRUE_CTR = {"hybrid": 0.65, "popular": 0.05}


def req(method: str, path: str, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(
        BASE + path, data=data, method=method,
        headers={"Content-Type": "application/json"} if body is not None else {},
    )
    with urllib.request.urlopen(r, timeout=60) as x:
        return json.load(x)


def main() -> None:
    rng = random.Random(SEED)
    req("PUT", "/v1/admin/strategies/auto", {"enabled": True})
    rows = []
    for i in range(1, ROUNDS + 1):
        resp = req("POST", "/v1/recommendations", {"client_id": CLIENT, "audience": "b2c"})
        arm = resp["strategy"]
        top = resp["recommendations"][0] if resp["recommendations"] else None
        clicked = 1 if top is not None and rng.random() < TRUE_CTR.get(arm, 0.1) else 0
        if clicked and top is not None:
            req("POST", "/v1/events", {
                "trace_id": resp["trace_id"], "client_id": CLIENT,
                "product_id": top["product_id"], "event": "clicked",
            })
        rows.append({
            "round": i, "strategy": arm,
            "product": top["title"] if top else "", "clicked": clicked,
        })
    req("PUT", "/v1/admin/strategies/auto", {"enabled": False})

    df = pd.DataFrame(rows)
    for arm in ["hybrid", "popular"]:
        df[f"is_{arm}"] = (df["strategy"] == arm).astype(float)
        df[f"share_{arm}"] = df[f"is_{arm}"].rolling(20, min_periods=1).mean()

    out = Path(__file__).with_name("live_bandit_simulation.csv")
    df.to_csv(out, index=False)
    print(df["strategy"].value_counts().to_string())
    print("last 20:")
    print(df.tail(20)["strategy"].value_counts(normalize=True).round(2).to_string())

    colors = {"hybrid": "#1d4ed8", "popular": "#c8102e"}
    fig, axes = plt.subplots(2, 1, figsize=(12, 8), sharex=True)
    for arm in ["hybrid", "popular"]:
        axes[0].plot(df["round"], df[f"share_{arm}"], label=arm, color=colors[arm], linewidth=2)
    axes[0].axvline(20, color="#6b7280", linestyle="--", linewidth=1)
    axes[0].text(23, 0.92, "cold start ends", color="#6b7280", fontsize=9)
    axes[0].set_ylabel("share of traffic (rolling 20)")
    axes[0].set_ylim(0, 1)
    axes[0].set_title("Live API: strategy bandit from cold start to stable")
    axes[0].legend(loc="center right")
    axes[0].grid(alpha=0.25)

    for arm in ["hybrid", "popular"]:
        sub = df[df["strategy"] == arm]
        if len(sub):
            axes[1].plot(sub["round"], sub["clicked"].expanding().mean(),
                         label=f"{arm} cumulative CTR", color=colors[arm], linewidth=2)
    axes[1].set_xlabel("session")
    axes[1].set_ylabel("cumulative CTR")
    axes[1].set_ylim(0, 1)
    axes[1].legend(loc="upper right")
    axes[1].grid(alpha=0.25)

    fig.tight_layout()
    png = Path(__file__).with_name("live_bandit_simulation.png")
    fig.savefig(png, dpi=160)
    print("saved", png)
    print("saved", out)


if __name__ == "__main__":
    main()
