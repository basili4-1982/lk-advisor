#!/usr/bin/env python3
"""Try to confirm adjacent_task candidates from their vendor page.

For every candidate with source='vendor' and type='adjacent_task' the script
fetches the evidence URL and looks for both product names plus an explicit
compatibility phrase. Confirmed candidates get vendor evidence and become
active; the rest stay candidates.
"""

from __future__ import annotations

import json
import os
import re

import psycopg2
import requests
from bs4 import BeautifulSoup

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120 Safari/537.36"
CONFIRM_WORDS = ["совмест", "комплект", "входит в состав", "вместе", "интегрир", "совместим"]


def page_text(url: str) -> str:
    r = requests.get(url, headers={"User-Agent": UA}, timeout=25)
    r.raise_for_status()
    html = r.content.decode("utf-8", "replace")
    soup = BeautifulSoup(html, "lxml")
    for tag in soup(["script", "style", "noscript", "svg"]):
        tag.decompose()
    return " ".join(soup.get_text(" ").split()).lower()


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("""
        SELECT r.id, r.from_item_id, i1.title, r.to_item_id, i2.title,
               COALESCE(r.evidence->0->'details'->>'url', ''),
               COALESCE(r.evidence->0->'details'->>'family', ''),
               COALESCE(r.evidence->0->'details'->>'edition_from', ''),
               COALESCE(r.evidence->0->'details'->>'edition_to', '')
        FROM relations r
        JOIN lk_items i1 ON i1.item_id = r.from_item_id
        JOIN lk_items i2 ON i2.item_id = r.to_item_id
        WHERE r.source = 'vendor' AND r.type = 'adjacent_task' AND r.status = 'candidate'
        ORDER BY r.id
    """)
    rows = cur.fetchall()
    cache: dict[str, str] = {}
    confirmed = needs_review = 0
    examples = []
    for rel_id, from_id, from_title, to_id, to_title, url, family, ea, eb in rows:
        if not url:
            needs_review += 1
            continue
        if url not in cache:
            try:
                cache[url] = page_text(url)
            except Exception:
                cache[url] = ""
        text = cache[url]
        def product_words(title: str) -> set[str]:
            words = set(re.findall(r"[a-z]+", title.lower()))
            return {w for w in words if w in {"desktop", "server", "enterprise", "workstation", "suite"}}

        a, b = product_words(from_title), product_words(to_title)
        both = bool(a) and bool(b) and a != b and all(w in text for w in (a | b))
        phrase = any(w in text for w in CONFIRM_WORDS)
        if both and phrase:
            evidence = json.dumps([{
                "kind": "vendor_doc",
                "summary": "Страница вендора подтверждает совместное использование",
                "details": {"url": url, "family": family},
            }], ensure_ascii=False)
            cur.execute("""
                UPDATE relations
                SET status = 'active', evidence = evidence || $2::jsonb, updated_at = now()
                WHERE id = $1
            """, (rel_id, evidence))
            confirmed += 1
            examples.append({"from": from_title, "to": to_title, "url": url})
        else:
            needs_review += 1
    conn.commit()
    print(json.dumps({
        "total": len(rows),
        "confirmed": confirmed,
        "needs_review": needs_review,
        "examples": examples[:3],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
