#!/usr/bin/env python3
"""Analyse the external-knowledge probe without calling the LLM again.

Reads analysis/external_probe_results.json and the catalog, then measures:
  * platform coverage that the internal catalog lacks;
  * edition-based relation candidates that do not exist internally;
  * stricter cross-vendor matches.
"""

from __future__ import annotations

import json
import os
import re
from collections import defaultdict
from typing import Any

import psycopg2

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
HERE = os.path.dirname(__file__)
RESULTS = os.path.join(HERE, "external_probe_results.json")
OUT = os.path.join(HERE, "external_probe_analysis.json")

GENERIC = {
    "система", "системы", "платформа", "сервис", "сервисы", "программа",
    "программы", "программного", "обеспечения", "решение", "решения",
    "windows", "linux", "android", "ios", "web", "mobile",
}


def norm(s: str) -> str:
    return re.sub(r"[^a-zа-я0-9]+", " ", s.lower()).strip()


def tokens(s: str) -> list[str]:
    return [t for t in norm(s).split() if len(t) >= 4 and t not in GENERIC]


def main() -> None:
    report = json.load(open(RESULTS, encoding="utf-8"))
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()

    cur.execute("SELECT from_item_id, to_item_id FROM relations")
    existing = set()
    for a, b in cur.fetchall():
        existing.add((a, b))
        existing.add((b, a))

    cur.execute("""
        SELECT count(*) FILTER (WHERE system_requirements <> '{}'::jsonb), count(*)
        FROM lk_items
    """)
    platform_internal, items_total = cur.fetchone()

    analysis: dict[str, Any] = {
        "internal_platform_items": platform_internal,
        "items_total": items_total,
        "families": [],
        "totals": {},
    }
    total_new = total_solo_edges = total_cross = 0

    for t in report["targets"]:
        family = t["family"]
        if not t["ok"]:
            continue
        cur.execute(
            """
            SELECT i.item_id, i.title,
                   EXISTS (SELECT 1 FROM relations r
                           WHERE r.from_item_id = i.item_id OR r.to_item_id = i.item_id)
            FROM lk_items i WHERE i.title ILIKE %s ORDER BY i.title
            """,
            (f"%{family}%",),
        )
        items = [
            {"item_id": r[0], "title": r[1], "has_relation": r[2]}
            for r in cur.fetchall()
        ]

        # Match each item to the editions named on the vendor page.
        editions = t.get("editions") or []
        edition_tokens = [(e, tokens(e)) for e in editions]
        matched: list[tuple[dict[str, Any], str]] = []
        for it in items:
            title_norm = norm(it["title"])
            for label, toks in edition_tokens:
                if any(tok in title_norm for tok in toks):
                    matched.append((it, label))
                    break

        # Only link items from different editions; exclude already known edges.
        candidates = []
        for i in range(len(matched)):
            for j in range(i + 1, len(matched)):
                a, ea = matched[i]
                b, eb = matched[j]
                if ea == eb:
                    continue
                key = (a["item_id"], b["item_id"])
                if key in existing:
                    continue
                candidates.append(
                    {
                        "from": a["item_id"],
                        "from_title": a["title"][:90],
                        "from_edition": ea,
                        "to": b["item_id"],
                        "to_title": b["title"][:90],
                        "to_edition": eb,
                        "type": "variant_of",
                        "reason": "вендор перечисляет разные редакции одной линейки",
                        "solo": (not a["has_relation"]) or (not b["has_relation"]),
                    }
                )
        # Prefer pairs that involve at least one solo item.
        candidates.sort(key=lambda c: not c["solo"])
        candidates = candidates[:30]

        # Stricter cross-vendor matching: 2+ significant tokens or a long exact
        # phrase, and never the same family/vendor.
        cross = []
        family_norm = norm(family)
        for mention in (t.get("integrates_with") or []) + (t.get("compatible_with") or []):
            toks = tokens(mention)
            if not toks:
                continue
            phrase = " ".join(toks)
            if len(toks) == 1 and len(toks[0]) < 6:
                continue
            if family_norm and family_norm in norm(mention):
                continue
            cur.execute(
                "SELECT item_id, title FROM lk_items WHERE title ILIKE %s LIMIT 5",
                (f"%{toks[-1]}%",),
            )
            for item_id, title in cur.fetchall():
                if family_norm and family_norm in norm(title):
                    continue
                if phrase not in norm(title) and len(toks) < 2:
                    continue
                cross.append(
                    {
                        "mention": mention,
                        "item_id": item_id,
                        "title": title[:90],
                        "type": "adjacent_task",
                        "reason": "внешняя страница упоминает интеграцию",
                    }
                )
        # De-duplicate.
        seen = set()
        cross = [c for c in cross if not (c["item_id"] in seen or seen.add(c["item_id"]))]

        total_new += len(candidates)
        total_solo_edges += sum(1 for c in candidates if c["solo"])
        total_cross += len(cross)

        analysis["families"].append(
            {
                "family": family,
                "url": t["url"],
                "items": len(items),
                "solo": sum(1 for x in items if not x["has_relation"]),
                "platforms": t["platforms"],
                "editions": editions,
                "items_matched_to_editions": len(matched),
                "new_candidates": len(candidates),
                "candidate_examples": candidates[:5],
                "cross_vendor": cross[:5],
            }
        )

    analysis["totals"] = {
        "families_with_data": len(analysis["families"]),
        "new_candidate_edges": total_new,
        "edges_involving_solo": total_solo_edges,
        "cross_vendor_candidates": total_cross,
        "platforms_extracted": sum(len(f["platforms"]) for f in analysis["families"]),
        "items_with_internal_platform": platform_internal,
        "items_total": items_total,
    }

    with open(OUT, "w", encoding="utf-8") as fh:
        json.dump(analysis, fh, ensure_ascii=False, indent=2)

    print(json.dumps(analysis["totals"], ensure_ascii=False, indent=2))
    for f in analysis["families"]:
        print(
            f"\n{f['family']}: items {f['items']}, solo {f['solo']}, "
            f"matched editions {f['items_matched_to_editions']}, "
            f"new {f['new_candidates']}, cross {len(f['cross_vendor'])}"
        )
        for c in f["candidate_examples"][:3]:
            print(f"  - {c['from_edition']} → {c['to_edition']}: {c['from_title'][:45]} | {c['to_title'][:45]}")
    print("\nsaved", OUT)


if __name__ == "__main__":
    main()
