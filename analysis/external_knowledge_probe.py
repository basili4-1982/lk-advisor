#!/usr/bin/env python3
"""Probe external vendor pages for knowledge the internal catalog lacks.

The script fetches a few vendor pages, asks the LLM to extract structured
facts (platforms, editions, integrations), matches them to catalog items and
reports whether those facts can produce new relation candidates.

Only extracted facts and URLs are written to the report; raw page text stays
out of the repository.
"""

from __future__ import annotations

import json
import os
import re
import sys
from dataclasses import dataclass, field
from typing import Any

import psycopg2
import requests
from bs4 import BeautifulSoup

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
LLM_BASE = os.environ.get("LLM_BASE_URL", "https://api.deepseek.com/v1").rstrip("/")
LLM_KEY = os.environ.get("LLM_API_KEY") or os.environ.get("DEEPSEEK_API_KEY", "")
LLM_MODEL = os.environ.get("LLM_MODEL", "deepseek-flash")

UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120 Safari/537.36"

TARGETS = [
    ("Astra Linux", "https://astralinux.ru/products/astra-linux-special-edition/"),
    ("Яндекс 360", "https://360.yandex.ru/business/"),
    ("КриптоПро", "https://www.cryptopro.ru/products/csp"),
    ("VK WorkMail", "https://workspace.vk.ru/"),
    ("1С-Битрикс", "https://www.1c-bitrix.ru/products/cms/"),
    ("The Bat!", "https://www.ritlabs.com/en/products/thebat/"),
    ("Dr.Web", "https://drweb.ru/"),
    ("Google Workspace", "https://workspace.google.com/pricing.html"),
]

SYSTEM = """Ты извлекаешь структурированные факты о программном продукте со страницы
вендора. Используй ТОЛЬКО текст страницы. Верни строго JSON без markdown:

{
  "vendor": "название вендора",
  "product": "название продукта или линейки",
  "category": "класс ПО по-русски",
  "platforms": ["Windows", "Linux", ...],
  "editions": ["названия редакций/тарифов"],
  "compatible_with": ["с чем совместим"],
  "integrates_with": ["с какими продуктами интегрируется"],
  "summary": "1-2 предложения о продукте"
}

Если данных нет — пустая строка или пустой массив. Не выдумывай."""


@dataclass
class PageFacts:
    family: str
    url: str
    ok: bool = False
    error: str = ""
    vendor: str = ""
    product: str = ""
    category: str = ""
    platforms: list[str] = field(default_factory=list)
    editions: list[str] = field(default_factory=list)
    compatible_with: list[str] = field(default_factory=list)
    integrates_with: list[str] = field(default_factory=list)
    summary: str = ""


def fetch_text(url: str) -> str:
    r = requests.get(url, headers={"User-Agent": UA}, timeout=25)
    r.raise_for_status()
    try:
        html = r.content.decode("utf-8")
    except UnicodeDecodeError:
        html = r.content.decode("cp1251", errors="replace")
    soup = BeautifulSoup(html, "lxml")
    for tag in soup(["script", "style", "noscript", "svg"]):
        tag.decompose()
    text = " ".join(soup.get_text(" ").split())
    return text[:16000]


def extract(text: str) -> dict[str, Any]:
    if not LLM_KEY:
        raise RuntimeError("LLM key is not set")
    body = {
        "model": LLM_MODEL,
        "temperature": 0,
        "max_tokens": 2500,
        "messages": [
            {"role": "system", "content": SYSTEM},
            {"role": "user", "content": "Текст страницы:\n\n" + text},
        ],
    }
    r = requests.post(
        LLM_BASE + "/chat/completions",
        headers={"Authorization": f"Bearer {LLM_KEY}", "Content-Type": "application/json"},
        json=body,
        timeout=60,
    )
    r.raise_for_status()
    content = r.json()["choices"][0]["message"]["content"].strip()
    if content.startswith("```"):
        content = re.sub(r"^```(?:json)?", "", content)
        content = re.sub(r"```$", "", content).strip()
    start, end = content.find("{"), content.rfind("}")
    if start >= 0 and end > start:
        content = content[start : end + 1]
    try:
        return json.loads(content)
    except json.JSONDecodeError:
        print("LLM raw:", content[:500], file=sys.stderr)
        raise


def norm(s: str) -> str:
    return re.sub(r"[^a-zа-я0-9]+", " ", s.lower()).strip()


def catalog_items(cur, family: str) -> list[dict[str, Any]]:
    cur.execute(
        """
        SELECT i.item_id, i.title, COALESCE(i.product_group, ''),
               EXISTS (
                   SELECT 1 FROM relations r
                   WHERE r.from_item_id = i.item_id OR r.to_item_id = i.item_id
               ) AS has_relation
        FROM lk_items i
        WHERE i.title ILIKE %s
        ORDER BY i.title
        """,
        (f"%{family}%",),
    )
    return [
        {"item_id": r[0], "title": r[1], "group": r[2], "has_relation": r[3]}
        for r in cur.fetchall()
    ]


def match_external_mentions(cur, mentions: list[str]) -> list[dict[str, str]]:
    """Find catalog items mentioned by an external page."""
    found: list[dict[str, str]] = []
    seen: set[str] = set()
    for mention in mentions:
        tokens = [t for t in norm(mention).split() if len(t) >= 3]
        if not tokens:
            continue
        # Use the most specific token to keep the query cheap.
        token = max(tokens, key=len)
        cur.execute(
            """
            SELECT item_id, title FROM lk_items
            WHERE title ILIKE %s
            ORDER BY title LIMIT 5
            """,
            (f"%{token}%",),
        )
        for item_id, title in cur.fetchall():
            if item_id in seen:
                continue
            seen.add(item_id)
            found.append({"item_id": item_id, "title": title, "mention": mention})
    return found


def main() -> int:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    facts: list[PageFacts] = []
    for family, url in TARGETS:
        f = PageFacts(family=family, url=url)
        try:
            text = fetch_text(url)
            data = extract(text)
            f.ok = True
            f.vendor = str(data.get("vendor", ""))
            f.product = str(data.get("product", ""))
            f.category = str(data.get("category", ""))
            f.platforms = [str(x) for x in data.get("platforms", []) if str(x).strip()]
            f.editions = [str(x) for x in data.get("editions", []) if str(x).strip()]
            f.compatible_with = [str(x) for x in data.get("compatible_with", []) if str(x).strip()]
            f.integrates_with = [str(x) for x in data.get("integrates_with", []) if str(x).strip()]
            f.summary = str(data.get("summary", ""))
        except Exception as exc:  # noqa: BLE001 - report any source failure
            f.error = str(exc)
        facts.append(f)
        print(
            f"{'OK ' if f.ok else 'ERR'} {family}: "
            + (f"{len(f.platforms)} platforms, {len(f.editions)} editions, "
               f"{len(f.integrates_with)} integrations" if f.ok else f.error),
            file=sys.stderr,
        )

    report: dict[str, Any] = {"targets": [], "totals": {}}
    total_items = total_solo = total_candidates = total_cross = 0
    total_platforms = total_editions = 0

    for f in facts:
        items = catalog_items(cur, f.family)
        solo = [x for x in items if not x["has_relation"]]
        cross = match_external_mentions(cur, f.integrates_with + f.compatible_with)

        # Candidate links: the page names editions of one family, so items from
        # that family can be linked as variants/alternatives even when their
        # product_group is unique and co-purchase is absent.
        candidates: list[dict[str, str]] = []
        if len(items) >= 2 and f.editions:
            for i in range(len(items)):
                for j in range(i + 1, len(items)):
                    candidates.append(
                        {
                            "from": items[i]["item_id"],
                            "to": items[j]["item_id"],
                            "type": "variant_of",
                            "reason": "внешняя страница перечисляет редакции одной линейки",
                        }
                    )
                    if len(candidates) >= 40:
                        break
                if len(candidates) >= 40:
                    break

        total_items += len(items)
        total_solo += len(solo)
        total_candidates += len(candidates)
        total_cross += len(cross)
        total_platforms += len(f.platforms)
        total_editions += len(f.editions)

        report["targets"].append(
            {
                "family": f.family,
                "url": f.url,
                "ok": f.ok,
                "error": f.error,
                "vendor": f.vendor,
                "product": f.product,
                "category": f.category,
                "platforms": f.platforms,
                "editions": f.editions,
                "compatible_with": f.compatible_with,
                "integrates_with": f.integrates_with,
                "summary": f.summary,
                "catalog_items": len(items),
                "catalog_solo": len(solo),
                "solo_examples": [x["title"] for x in solo[:3]],
                "candidate_links": len(candidates),
                "candidate_examples": candidates[:3],
                "external_matches": cross[:5],
            }
        )

    report["totals"] = {
        "pages_ok": sum(1 for f in facts if f.ok),
        "pages_total": len(facts),
        "catalog_items": total_items,
        "catalog_solo": total_solo,
        "platforms_extracted": total_platforms,
        "editions_extracted": total_editions,
        "candidate_links": total_candidates,
        "cross_vendor_matches": total_cross,
    }

    out = os.path.join(os.path.dirname(__file__), "external_probe_results.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=2)

    print(json.dumps(report["totals"], ensure_ascii=False, indent=2))
    print("saved", out)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
