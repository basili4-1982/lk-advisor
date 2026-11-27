#!/usr/bin/env python3
"""Import the official Russian software registry XLSX export into vendor_products.

The export has a title block, a header on row 5 and one record per row. Excluded
records (with an exclusion date) are skipped. Duplicate vendor/product rows keep
the record with the highest registry number.
"""

from __future__ import annotations

import json
import os
import re
import sys
from collections import OrderedDict

import openpyxl
import psycopg2
from psycopg2.extras import execute_values

DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
DEFAULT = "/home/basili4/Загрузки/Экспорт Реестра (Реестр российского ПО) от 2026_11_27-00-34.xlsx"


def clean(value) -> str:
    if value is None:
        return ""
    return re.sub(r"\s+", " ", str(value)).strip()


def registry_num(value) -> int:
    m = re.search(r"\d+", clean(value))
    return int(m.group(0)) if m else 0


def main() -> int:
    path = sys.argv[1] if len(sys.argv) > 1 else DEFAULT
    wb = openpyxl.load_workbook(path, read_only=True, data_only=True)
    ws = wb[wb.sheetnames[0]]
    rows = ws.iter_rows(values_only=True)
    header = None
    for row in rows:
        if row and clean(row[0]) == "Регистрационный номер":
            header = [clean(x) for x in row]
            break
    if not header:
        print("header not found", file=sys.stderr)
        return 1
    idx = {name: i for i, name in enumerate(header)}

    def col(row, name):
        i = idx.get(name, -1)
        return row[i] if 0 <= i < len(row) else None

    records: "OrderedDict[tuple[str, str], dict]" = OrderedDict()
    total = active = 0
    for row in rows:
        if not row or not clean(col(row, "Регистрационный номер")):
            continue
        total += 1
        if clean(col(row, "Дата исключения")):
            continue
        product = clean(col(row, "Наименование ПО"))
        if not product:
            continue
        vendor = clean(col(row, "Сокращенное наименование (ФИО) правообладателя")) or clean(
            col(row, "Наименование (ФИО) правообладателя")
        )
        if vendor in ("", "-"):
            continue
        key = (vendor.lower(), product.lower())
        num = registry_num(col(row, "Регистрационный номер"))
        if key in records and records[key]["num"] >= num:
            continue
        aliases = [
            a.strip()
            for a in re.split(r"[\n;]+", clean(col(row, "Альтернативные названия ПО")))
            if a.strip()
        ]
        url = clean(col(row, "Сайт с документацией по установке и эксплуатации ПО"))
        if not url:
            url = clean(col(row, "Ссылка на решения уполномоченного органа о включении сведений о ПО"))
        records[key] = {
            "vendor": vendor,
            "product": product,
            "category": clean(col(row, "Класс ПО")),
            "registry": clean(col(row, "Регистрационный номер")),
            "num": num,
            "url": url,
            "aliases": aliases,
        }
        active += 1

    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("SELECT lower(vendor), lower(product) FROM vendor_products WHERE registry_number <> ''")
    existing = {(v, p) for v, p in cur.fetchall()}

    values = []
    for (vendor_key, product_key), r in records.items():
        values.append((
            r["vendor"], r["product"], r["category"], "", "[]",
            r["registry"], r["url"], json.dumps(r["aliases"], ensure_ascii=False),
        ))
    execute_values(cur, """
        INSERT INTO vendor_products
            (vendor, product, category, edition, platforms, registry_number, source_url, aliases)
        VALUES %s
        ON CONFLICT (vendor, product, edition) DO UPDATE SET
            category = EXCLUDED.category,
            registry_number = EXCLUDED.registry_number,
            source_url = EXCLUDED.source_url,
            aliases = EXCLUDED.aliases,
            updated_at = now()
    """, values, page_size=1000)
    conn.commit()

    added = sum(1 for v, p in records if (v, p) not in existing)
    updated = len(records) - added
    with_url = sum(1 for r in records.values() if r["url"])
    with_alias = sum(1 for r in records.values() if r["aliases"])
    print(json.dumps({
        "rows": total,
        "active_rows": active,
        "unique_products": len(records),
        "added": added,
        "updated": updated,
        "with_url": with_url,
        "with_aliases": with_alias,
    }, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
