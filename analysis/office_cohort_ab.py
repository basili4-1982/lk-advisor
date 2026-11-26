#!/usr/bin/env python3
"""Cohort A/B on agents with the same hidden persona.

20 synthetic clients share a hidden "Go backend developer" profile.
The profile is never sent to the service. Each client has its own history and
events; the recommender sees only client_id + purchases/events.

Arm A: 10 clients, forced `popular`.
Arm B: 10 clients, `hybrid` + feedback + exploration.
"""

from __future__ import annotations

import json
import math
import random
import subprocess
import urllib.request
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd

BASE = "http://localhost:8080"
ROUNDS = 100
AGENTS_PER_ARM = 10
SEED = 23
CLIENTS = [f"smb-{i:03d}" for i in range(1, 2 * AGENTS_PER_ARM + 1)]

PERSONA = {
    "name": "Office worker / accountant",
    "category": {
        "office": 1.0, "training": 0.7, "backup": 0.55,
        "security": 0.4, "analytics": 0.2, "devtools": 0.1,
        "games": -1.0, "graphics": -0.6, "cad": -0.6,
    },
    "keywords": {
        "office": 0.4, "pdf": 0.5, "документ": 0.5, "курс": 0.4,
        "security": 0.3, "backup": 0.3, "game": -0.8,
        "corel": -0.5, "cad": -0.5,
    },
    "preferred": {"office", "training", "backup", "security"},
}


def req(method: str, path: str, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(
        BASE + path, data=data, method=method,
        headers={"Content-Type": "application/json"} if body is not None else {},
    )
    with urllib.request.urlopen(r, timeout=60) as x:
        return json.load(x)


def utility(product: dict) -> float:
    text = " " + product["title"].lower() + " "
    score = PERSONA["category"].get(product["category"], 0.0)
    for word, boost in PERSONA["keywords"].items():
        if word in text:
            score += boost
    score -= product["price"] / 100000 * 0.05
    return score


def setup_client(client_id: str):
    req("POST", f"/v1/demo/clients/{client_id}/reset", {})
    req("POST", f"/v1/demo/clients/{client_id}/items", {"product_id": "prod-office-suite"})


def run_agent(client_id: str, arm: str, products: dict[str, dict], seed: int) -> pd.DataFrame:
    rng = random.Random(seed)
    setup_client(client_id)
    rows = []
    for session in range(1, ROUNDS + 1):
        resp = req("POST", "/v1/recommendations", {"client_id": client_id, "strategy": arm})
        top = resp.get("recommendations", [])[:3]
        scored = [(utility(products.get(x["product_id"], {"title": x["title"], "category": "", "price": 0})), x)
                  for x in top]
        scored.sort(key=lambda x: -x[0])
        best_u, best = scored[0] if scored else (0, None)
        click_prob = 1 / (1 + math.exp(-1.5 * best_u)) if best else 0
        clicked = best is not None and rng.random() < click_prob
        purchase_prob = 1 / (1 + math.exp(-2.0 * (best_u - 0.4))) if best else 0
        purchased = clicked and rng.random() < purchase_prob
        if clicked:
            req("POST", "/v1/events", {
                "trace_id": resp["trace_id"], "client_id": client_id,
                "product_id": best["product_id"], "event": "clicked",
            })
        if purchased:
            req("POST", "/v1/events", {
                "trace_id": resp["trace_id"], "client_id": client_id,
                "product_id": best["product_id"], "event": "purchased",
            })
        match = sum(min(1.0, max(0, u)) for u, _ in scored) / max(1, len(scored))
        preferred = sum(1 for _, x in scored
                        if products.get(x["product_id"], {}).get("category") in PERSONA["preferred"]) / max(1, len(scored))
        rows.append({
            "client_id": client_id, "arm": arm, "session": session,
            "match": match, "preferred_share": preferred,
            "clicked": int(clicked), "purchased": int(purchased),
        })
    return pd.DataFrame(rows)


def main() -> None:
    products = {p["id"]: p for p in req("GET", "/v1/products")}
    req("PUT", "/v1/admin/strategies/auto", {"enabled": False})

    rows = []
    for i, cid in enumerate(CLIENTS):
        arm = "popular" if i < AGENTS_PER_ARM else "hybrid"
        rows.append(run_agent(cid, arm, products, SEED + i))
    df = pd.concat(rows, ignore_index=True)

    out = Path(__file__).with_name("office_cohort_ab.csv")
    df.to_csv(out, index=False)

    # Aggregate across agents per session.
    agg = df.groupby(["arm", "session"]).agg(
        match=("match", "mean"),
        match_se=("match", lambda x: x.std(ddof=1) / math.sqrt(len(x))),
        preferred=("preferred_share", "mean"),
        preferred_se=("preferred_share", lambda x: x.std(ddof=1) / math.sqrt(len(x))),
    ).reset_index()

    fig, axes = plt.subplots(2, 1, figsize=(12, 8), sharex=True)
    for arm, color in [("popular", "#c8102e"), ("hybrid", "#1d4ed8")]:
        sub = agg[agg.arm == arm].copy()
        for col, se_col, ax in [("match", "match_se", axes[0]),
                                 ("preferred", "preferred_se", axes[1])]:
            roll = sub[col].rolling(10, min_periods=1).mean()
            se = sub[se_col].rolling(10, min_periods=1).mean()
            ax.plot(sub.session, roll, label=f"{arm} · {col}", color=color, linewidth=2)
            ax.fill_between(sub.session, roll - 1.96 * se, roll + 1.96 * se,
                            color=color, alpha=0.12)
    axes[0].set_ylabel("persona match (rolling 10, mean ±95% CI)")
    axes[0].set_ylim(0, 1)
    axes[0].set_title(f"Cohort A/B on agents: {AGENTS_PER_ARM} clients per arm, hidden office-worker persona")
    axes[0].legend()
    axes[0].grid(alpha=0.25)
    axes[1].set_xlabel("session per agent")
    axes[1].set_ylabel("preferred category share")
    axes[1].set_ylim(0, 1)
    axes[1].legend()
    axes[1].grid(alpha=0.25)
    fig.tight_layout()
    png = Path(__file__).with_name("office_cohort_ab.png")
    fig.savefig(png, dpi=160)
    print("saved", png)
    print("saved", out)

    summary = df.groupby("arm").agg(
        match=("match", "mean"), preferred=("preferred_share", "mean"),
        clicks=("clicked", "sum"), purchases=("purchased", "sum"),
        agents=("client_id", "nunique"),
    ).reset_index()
    print(summary.to_string(index=False))

    # Restore the demo seed after the experiment.
    subprocess.run(
        ["go", "run", "./cmd/lk-advisor", "seed", "--seed", "42"],
        cwd=Path(__file__).resolve().parents[1],
        env={**__import__("os").environ, "DATABASE_URL": "postgres://lk:lk@localhost:55432/lk?sslmode=disable"},
        check=False,
    )


if __name__ == "__main__":
    main()
