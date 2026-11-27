#!/usr/bin/env python3
"""Download and import the FSTEC certified security tools registry (ODS)."""

from __future__ import annotations

import csv
import datetime as dt
import json
import os
import subprocess
import tempfile
import urllib3

import psycopg2
import requests
from psycopg2.extras import execute_values

urllib3.disable_warnings()
DSN = os.environ.get(
    "DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
)
URL = ("https://reestr.fstec.ru/?option=com_rajax&module=rfiles&method=download"
       "&format=file&mod=209&file=0")
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36"


def parse_date(raw: str):
    raw = (raw or "").strip()
    if not raw:
        return None
    for fmt in ("%d.%m.%Y", "%Y-%m-%d"):
        try:
            return dt.datetime.strptime(raw, fmt).date()
        except ValueError:
            pass
    return None


def main() -> int:
    tmp = tempfile.mkdtemp(prefix="fstec_")
    ods = os.path.join(tmp, "fstec.ods")
    r = requests.get(URL, headers={"User-Agent": UA}, verify=False, timeout=90)
    r.raise_for_status()
    with open(ods, "wb") as fh:
        fh.write(r.content)
    subprocess.run(
        ["soffice", "--headless", "--convert-to", "csv", "--outdir", tmp, ods],
        check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=180,
    )
    csv_path = os.path.join(tmp, "fstec.csv")
    with open(csv_path, encoding="utf-8", errors="replace", newline="") as fh:
        rows = list(csv.reader(fh))
    header = rows[0]
    idx = {name: i for i, name in enumerate(header)}

    def get(row, name):
        i = idx.get(name, -1)
        return row[i].strip() if 0 <= i < len(row) else ""

    conn = psycopg2.connect(DSN)
    cur = conn.cursor()
    cur.execute("SELECT certificate_number, product, applicant FROM fstec_certificates")
    existing = set(cur.fetchall())
    values = []
    for row in rows[1:]:
        product = get(row, "Наименование средства (шифр)")
        number = get(row, "№ сертификата")
        if not product or not number:
            continue
        values.append((
            number,
            parse_date(get(row, "Дата внесения в реестр")),
            parse_date(get(row, "Срок действия сертификата")),
            product,
            get(row, "Наименования документов, требованиям которых соответствует средство"),
            get(row, "Схема сертификации"),
            get(row, "Испытательная лаборатория"),
            get(row, "Орган по сертификации"),
            get(row, "Заявитель"),
            get(row, "Реквизиты заявителя (индекс, адрес, телефон)"),
            parse_date(get(row, "Информация об окончании срока технической поддержки, полученная от заявителя")),
            get(row, "Сведения о поддержке безопасности"),
        ))
    execute_values(cur, """
        INSERT INTO fstec_certificates
            (certificate_number, registered_at, valid_until, product, requirements,
             scheme, lab, body, applicant, address, support_until, security_support)
        VALUES %s
        ON CONFLICT (certificate_number, product, applicant) DO UPDATE SET
            registered_at = EXCLUDED.registered_at,
            valid_until = EXCLUDED.valid_until,
            requirements = EXCLUDED.requirements,
            scheme = EXCLUDED.scheme,
            lab = EXCLUDED.lab,
            body = EXCLUDED.body,
            address = EXCLUDED.address,
            support_until = EXCLUDED.support_until,
            security_support = EXCLUDED.security_support,
            updated_at = now()
    """, values, page_size=500)
    conn.commit()

    added = sum(1 for v in values if (v[0], v[3], v[8]) not in existing)
    print(json.dumps({
        "rows": len(rows) - 1,
        "imported": len(values),
        "added": added,
        "updated": len(values) - added,
    }, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
