#!/usr/bin/env python3
"""Vertical-slice experiment demo on synthetic clients.

Resets the demo hypothesis, assigns clients deterministically, runs API
recommendations and records clicks/purchases for the experiment target.
The data is synthetic; no raw dataset rows are written to the repository.
"""

from __future__ import annotations

import json
import random
import urllib.request

BASE = "http://localhost:8080"
TARGET = "prod-pdf"
SESSIONS = 10
SEED = 7


def req(method: str, path: str, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(
        BASE + path, data=data, method=method,
        headers={"Content-Type": "application/json"} if body is not None else {},
    )
    with urllib.request.urlopen(r, timeout=60) as x:
        return json.load(x)


def main() -> None:
    req("POST", "/v1/admin/experiments/demo", {})
    clients = [
        c["id"] for c in req("GET", "/v1/admin/state")["clients"]
        if c["id"].startswith("smb-")
    ]
    rng = random.Random(SEED)
    sessions = 0
    target_shown = 0
    clicks = 0
    purchases = 0

    for cid in clients:
        for _ in range(SESSIONS):
            resp = req("POST", "/v1/recommendations", {"client_id": cid, "strategy": "hybrid"})
            sessions += 1
            top = resp.get("recommendations", [])
            found = next((x for x in top if x["product_id"] == TARGET), None)
            if not found:
                continue
            target_shown += 1
            if rng.random() < 0.75:
                clicks += 1
                req("POST", "/v1/events", {
                    "trace_id": resp["trace_id"], "client_id": cid,
                    "product_id": TARGET, "event": "clicked",
                })
                if rng.random() < 0.30:
                    purchases += 1
                    req("POST", "/v1/events", {
                        "trace_id": resp["trace_id"], "client_id": cid,
                        "product_id": TARGET, "event": "purchased",
                    })

    print(f"clients={len(clients)} sessions={sessions} target_shown={target_shown} "
          f"clicks={clicks} purchases={purchases}")

    experiments = req("GET", "/v1/admin/experiments")["items"]
    exp = next((x for x in experiments if x["id"] == "exp-pdf-b2b"), None)
    if not exp:
        print("experiment not found")
        return
    print(f"\n{exp['title']}\n{exp['hypothesis']}\n")
    for r in exp.get("results", []):
        print(
            f"{r['variant']:10s} exposures={r['exposures']:4d} clients={r['clients']:3d} "
            f"clicks={r['clicks']:4d} purchases={r['purchases']:4d} "
            f"CTR={r['ctr']*100:5.1f}% conversion={r['conversion']*100:5.1f}%"
        )


if __name__ == "__main__":
    main()
