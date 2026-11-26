#!/usr/bin/env python3
"""Measure how fast a wrong hypothesis is stopped by guardrails.

Runs the office-worker persona against a deliberately bad experiment
(CorelDRAW for office workers), checks guardrails every N sessions and calls
the monitor endpoint. Synthetic demo data only.
"""

from __future__ import annotations

import json
import math
import random
import urllib.request

BASE = "http://localhost:8080"
BAD_ID = "exp-coreldraw-bad"
SESSIONS = 1000
CHUNK = 100
SEED = 19

PERSONA = {
    "category": {"office": 1.0, "training": 0.7, "backup": 0.55,
                 "security": 0.4, "analytics": 0.2, "devtools": 0.1,
                 "games": -1.0, "graphics": -0.8, "cad": -0.8},
    "keywords": {"office": 0.4, "pdf": 0.5, "документ": 0.5, "курс": 0.4,
                 "security": 0.3, "backup": 0.3, "game": -0.8,
                 "corel": -0.9, "cad": -0.8},
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
    score = PERSONA["category"].get(product.get("category", ""), 0.0)
    for word, boost in PERSONA["keywords"].items():
        if word in text:
            score += boost
    price = product.get("price", 0)
    if isinstance(price, dict):
        price = price.get("amount", 0)
    score -= price / 100000 * 0.05
    return score


def main() -> None:
    req("POST", "/v1/admin/experiments/bad-demo", {})
    clients = [c["id"] for c in req("GET", "/v1/admin/state")["clients"] if c["id"].startswith("smb-")]
    rng = random.Random(SEED)
    sessions_run = 0
    checkpoints = []

    for chunk_start in range(0, SESSIONS, CHUNK):
        for i in range(CHUNK):
            cid = clients[(chunk_start + i) % len(clients)]
            resp = req("POST", "/v1/recommendations", {"client_id": cid, "strategy": "hybrid"})
            top = resp.get("recommendations", [])
            if not top:
                continue
            scored = [(utility(x), x) for x in top]
            scored.sort(key=lambda x: -x[0])
            best_u, best = scored[0]
            click_p = 1 / (1 + math.exp(-1.5 * best_u))
            if rng.random() < click_p:
                req("POST", "/v1/events", {
                    "trace_id": resp["trace_id"], "client_id": cid,
                    "product_id": best["product_id"], "event": "clicked",
                })
                purchase_p = 1 / (1 + math.exp(-2.0 * (best_u - 0.4)))
                if rng.random() < purchase_p:
                    req("POST", "/v1/events", {
                        "trace_id": resp["trace_id"], "client_id": cid,
                        "product_id": best["product_id"], "event": "purchased",
                    })
            sessions_run += 1

        exp = next((x for x in req("GET", "/v1/admin/experiments")["items"] if x["id"] == BAD_ID), None)
        if not exp:
            break
        c = next((r for r in exp["results"] if r["variant"] == "control"), {})
        t = next((r for r in exp["results"] if r["variant"] == "treatment"), {})
        monitor = req("POST", f"/v1/admin/experiments/{BAD_ID}/monitor", {})
        checkpoint = {
            "sessions": sessions_run,
            "control_exposures": c.get("exposures", 0),
            "treatment_exposures": t.get("exposures", 0),
            "control_conversion": c.get("conversion", 0),
            "treatment_conversion": t.get("conversion", 0),
            "delta": t.get("conversion", 0) - c.get("conversion", 0),
            "guardrail": monitor.get("outcome", {}).get("guardrail", {}),
            "stopped": monitor.get("stopped", False),
        }
        checkpoints.append(checkpoint)
        print(
            f"sessions={sessions_run:4d} exp={c.get('exposures',0):3d}/{t.get('exposures',0):3d} "
            f"conv={c.get('conversion',0)*100:5.1f}%/{t.get('conversion',0)*100:5.1f}% "
            f"delta={(t.get('conversion',0)-c.get('conversion',0))*100:+5.2f}pp "
            f"blocked={checkpoint['guardrail'].get('blocked')} suggested={checkpoint['guardrail'].get('suggested')} stopped={checkpoint['stopped']}"
        )
        if checkpoint["stopped"]:
            break

    print("\ncheckpoints:", len(checkpoints))
    print("stopped_at_sessions:", checkpoints[-1]["sessions"] if checkpoints and checkpoints[-1]["stopped"] else None)


if __name__ == "__main__":
    main()
