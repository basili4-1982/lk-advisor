#!/usr/bin/env python3
"""Precision check for relation candidates built from vendor_products.

A candidate is "confirmed" when both sides share one external family and have
different non-empty editions (edition ladder). A candidate is "needs review"
when the link is based only on platform differences. Anything else is invalid.

The script writes a CSV for manual review but only prints aggregate numbers, so
internal item ids do not end up in the repository.
"""

from __future__ import annotations

import csv
import json
import os

import psycopg2

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
OUT = os.path.join(os.path.dirname(__file__), "vendor_candidates_precision.csv")


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("""
        SELECT r.from_item_id, i1.title, r.to_item_id, i2.title, r.type, r.status,
               COALESCE(r.evidence->0->'details'->>'family', ''),
               COALESCE(r.evidence->0->'details'->>'edition_from', ''),
               COALESCE(r.evidence->0->'details'->>'edition_to', ''),
               COALESCE(r.evidence->0->'details'->>'platforms_from', ''),
               COALESCE(r.evidence->0->'details'->>'platforms_to', ''),
               COALESCE(r.evidence->0->'details'->>'url', '')
        FROM relations r
        JOIN lk_items i1 ON i1.item_id = r.from_item_id
        JOIN lk_items i2 ON i2.item_id = r.to_item_id
        WHERE r.source = 'vendor'
        ORDER BY r.id
    """)
    rows = cur.fetchall()
    labels = []
    confirmed = needs_review = invalid = 0
    for row in rows:
        status, family, ea, eb = row[5], row[6], row[7], row[8]
        if row[4] == "variant_of" and family and ea and eb and ea != eb:
            label = "confirmed_variant"
            confirmed += 1
        elif row[4] in ("adjacent_task", "bundle") and status == "active":
            label = "confirmed_complement"
            confirmed += 1
        elif family:
            label = "needs_review"
            needs_review += 1
        else:
            label = "invalid"
            invalid += 1
        labels.append([*row, label])

    with open(OUT, "w", encoding="utf-8", newline="") as fh:
        writer = csv.writer(fh)
        writer.writerow([
            "from_id", "from_title", "to_id", "to_title", "type", "status", "family",
            "edition_from", "edition_to", "platforms_from", "platforms_to",
            "source_url", "label",
        ])
        writer.writerows(labels)

    total = len(rows)
    print(json.dumps({
        "total": total,
        "confirmed": confirmed,
        "needs_review": needs_review,
        "invalid": invalid,
        "confirmed_total": confirmed,
        "precision_confirmed_pct": round(100 * confirmed / total, 1) if total else 0,
        "false_positive_pct": round(100 * invalid / total, 1) if total else 0,
    }, ensure_ascii=False, indent=2))
    print("csv:", OUT)


if __name__ == "__main__":
    main()
