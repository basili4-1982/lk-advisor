#!/usr/bin/env python3
"""A/B on agents: a hidden Go-developer persona drives real API sessions.

The persona is never sent to the recommendation service. The service sees only
the client id, two starting purchases (IDE and CI Runner) and click/purchase
events.

Arm A: forced `popular` strategy.
Arm B: `hybrid` strategy with feedback and exploration.

The script measures how well the top-3 match the hidden persona.
"""

from __future__ import annotations

import json
import math
import random
import urllib.request
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd

BASE = "http://localhost:8080"
CLIENT = "smb-006"
ROUNDS = 100
SEED = 11

# Hidden profile. It is NOT sent to the service.
PERSONA = {
    "name": "Go backend developer",
    "category": {
        "devtools": 1.0, "security": 0.9, "backup": 0.85,
        "training": 0.5, "office": 0.3, "analytics": 0.2,
        "virtualization": 0.2, "communication": 0.1,
        "games": -1.0, "graphics": -0.8, "cad": -0.6,
    },
    "keywords": {
        "ide": 0.5, " ci ": 0.5, "ci runner": 0.6, "security": 0.4,
        "backup": 0.4, "pdf": 0.2, "course": 0.1, "go": 0.4,
        "game": -0.8, "corel": -0.5, "cad": -0.5,
    },
    "preferred": {"devtools", "security", "backup", "training"},
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


def setup_client():
    req("POST", f"/v1/demo/clients/{CLIENT}/reset", {})
    for pid in ("prod-ide", "prod-ci"):
        req("POST", f"/v1/demo/clients/{CLIENT}/items", {"product_id": pid})


def run_arm(strategy: str, products: dict[str, dict], rng: random.Random) -> pd.DataFrame:
    setup_client()
    rows = []
    for i in range(1, ROUNDS + 1):
        resp = req("POST", "/v1/recommendations", {
            "client_id": CLIENT, "strategy": strategy,
        })
        top = resp.get("recommendations", [])[:3]
        scored = [(utility(products.get(x["product_id"], {"title": x["title"], "category": "", "price": 0})), x)
                  for x in top]
        scored.sort(key=lambda x: -x[0])
        best_utility, best = scored[0] if scored else (0, None)
        click_prob = 1 / (1 + math.exp(-1.5 * best_utility)) if best else 0
        clicked = best is not None and rng.random() < click_prob
        purchase_prob = 1 / (1 + math.exp(-2.0 * (best_utility - 0.4))) if best else 0
        purchased = clicked and rng.random() < purchase_prob
        if clicked:
            req("POST", "/v1/events", {
                "trace_id": resp["trace_id"], "client_id": CLIENT,
                "product_id": best["product_id"], "event": "clicked",
            })
        if purchased:
            req("POST", "/v1/events", {
                "trace_id": resp["trace_id"], "client_id": CLIENT,
                "product_id": best["product_id"], "event": "purchased",
            })
        match = sum(max(0, u) for u, _ in scored) / max(1, len(scored))
        preferred = sum(1 for _, x in scored
                        if products.get(x["product_id"], {}).get("category") in PERSONA["preferred"]) / max(1, len(scored))
        rows.append({
            "arm": strategy, "session": i,
            "match": match, "preferred_share": preferred,
            "clicked": int(clicked), "purchased": int(purchased),
            "top_category": products.get(best["product_id"], {}).get("category", "") if best else "",
        })
    return pd.DataFrame(rows)


def main() -> None:
    products = {p["id"]: p for p in req("GET", "/v1/products")}
    req("PUT", "/v1/admin/strategies/auto", {"enabled": False})
    rng = random.Random(SEED)
    control = run_arm("popular", products, rng)
    adaptive = run_arm("hybrid", products, rng)
    req("POST", f"/v1/demo/clients/{CLIENT}/reset", {})

    all_df = pd.concat([control, adaptive], ignore_index=True)
    out = Path(__file__).with_name("hidden_persona_ab.csv")
    all_df.to_csv(out, index=False)

    fig, axes = plt.subplots(2, 1, figsize=(12, 8), sharex=True)
    for arm, color in [("popular", "#c8102e"), ("hybrid", "#1d4ed8")]:
        sub = all_df[all_df.arm == arm]
        axes[0].plot(sub.session, sub.match.rolling(10, min_periods=1).mean(),
                     label=f"{arm} · match", color=color, linewidth=2)
        axes[1].plot(sub.session, sub.preferred_share.rolling(10, min_periods=1).mean(),
                     label=f"{arm} · preferred share", color=color, linewidth=2)
    axes[0].set_ylabel("persona match (rolling 10)")
    axes[0].set_ylim(0, 1)
    axes[0].set_title("A/B on agents: hidden Go-developer persona")
    axes[0].legend()
    axes[0].grid(alpha=0.25)
    axes[1].set_xlabel("session")
    axes[1].set_ylabel("preferred category share")
    axes[1].set_ylim(0, 1)
    axes[1].legend()
    axes[1].grid(alpha=0.25)
    fig.tight_layout()
    png = Path(__file__).with_name("hidden_persona_ab.png")
    fig.savefig(png, dpi=160)
    print("saved", png)
    print("saved", out)
    for arm, df in [("popular", control), ("hybrid", adaptive)]:
        first, last = df.head(10), df.tail(10)
        print(f"\n{arm}: match first10={first.match.mean():.3f} last10={last.match.mean():.3f} "
              f"preferred first10={first.preferred_share.mean():.3f} last10={last.preferred_share.mean():.3f} "
              f"clicks={df.clicked.sum()} purchases={df.purchased.sum()}")


if __name__ == "__main__":
    main()
