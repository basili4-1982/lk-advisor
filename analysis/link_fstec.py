#!/usr/bin/env python3
"""Match FSTEC certificates to catalog items.

Two passes:
  1. exact normalized product phrase inside the item title;
  2. fallback by rare distinctive tokens (document frequency <= 10).
The fallback is capped so one certificate cannot flood the catalog.
"""

from __future__ import annotations

import json
import os
import re
from collections import Counter

import psycopg2
from psycopg2.extras import execute_values

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
STOP = {
    "программное", "обеспечение", "обеспечения", "программного", "программный",
    "комплекс", "комплекса", "система", "системы", "средство", "средства",
    "защиты", "защита", "информации", "информация", "данных", "версия",
    "версии", "модуль", "модуля", "устройство", "устройства", "для", "или",
    "средств", "комплект", "пакет", "базовый", "специальное", "специального",
    "россии", "россия", "standard", "enterprise", "edition", "server",
    "windows", "linux", "platform", "security", "cloud", "database",
    "professional", "business", "system", "certified", "service", "suite",
    "license", "лицензия", "редакция", "сервер", "платформа", "облако",
    "база", "доступа", "несанкционированного", "конфиденциальной", "уровня",
    "класса", "сертификат", "соответствия", "требований", "russian",
    "renewal", "endpoint", "desktop", "device", "mobile", "special",
    "workspace", "monthly", "использование", "использования", "предоставление",
    "сервисам", "учетную", "месяц", "продление", "программы", "лицензий",
    "назначения", "операционную", "запись",
}


def norm(s: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"[^a-zа-я0-9]+", " ", (s or "").lower())).strip()


def phrase(product: str) -> str:
    m = re.search(r"[«\"]([^»\"]{8,})[»\"]", product or "")
    if m:
        product = m.group(1)
    product = re.sub(
        r"^(программное изделие|программный комплекс|программное обеспечение|"
        r"система|средство|комплекс|модуль|устройство)\s+",
        "", product, flags=re.I,
    )
    return norm(product)


def main() -> None:
    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("DELETE FROM fstec_certificate_items")

    cur.execute("SELECT id, product FROM fstec_certificates")
    certs = [(cid, phrase(product)) for cid, product in cur.fetchall()]
    cur.execute("SELECT item_id, title FROM lk_items")
    items = []
    for iid, title in cur.fetchall():
        n = norm(title)
        items.append((iid, n, {w for w in n.split() if len(w) >= 5}))
    df = Counter()
    for _, _, ts in items:
        df.update(ts)

    mappings = []
    methods = Counter()
    matched_certs = set()
    matched_items = set()
    for cid, ph in certs:
        matched: list[tuple[str, str, float]] = []
        if ph:
            for iid, title_norm, _ in items:
                if ph in title_norm:
                    matched.append((iid, "fstec_phrase", 0.85))
        if not matched:
            rare = {w for w in ph.split() if len(w) >= 6 and w not in STOP and df.get(w, 999) <= 10}
            if rare:
                for iid, _, ts in items:
                    if len(rare & ts) >= 2:
                        matched.append((iid, "fstec_token", 0.65))
                        if len(matched) >= 8:
                            break
        for iid, method, confidence in matched:
            mappings.append((cid, iid, method, confidence))
            methods[method] += 1
            matched_certs.add(cid)
            matched_items.add(iid)

    execute_values(cur, """
        INSERT INTO fstec_certificate_items (certificate_id, item_id, method, confidence)
        VALUES %s
        ON CONFLICT DO NOTHING
    """, mappings, page_size=1000)
    conn.commit()

    cur.execute("SELECT count(*), count(*) FILTER (WHERE valid_until < current_date) FROM fstec_certificates")
    total, expired = cur.fetchone()
    print(json.dumps({
        "certificates": total,
        "expired": expired,
        "matched_certificates": len(matched_certs),
        "matched_items": len(matched_items),
        "mappings": len(mappings),
        "phrase": methods["fstec_phrase"],
        "token": methods["fstec_token"],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
