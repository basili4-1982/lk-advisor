# Сквозной кейс: профиль → связи → рекомендация

Один клиент, один запрос, вся цепочка. Без UI: на входе JSON, на выходе JSON.

## 1. Клиент

SMB, e-commerce, 50–200 сотрудников, 18 месяцев в базе.

```json
{
  "client_id": "smb-001",
  "facts": {
    "segment": "SMB",
    "industry": "e-commerce",
    "size": "50-200",
    "tenure_months": 18,
    "active_licenses": 3,
    "renewals_next_90d": 2,
    "events_30d": 12,
    "environment": {
      "os": "Windows 11",
      "ram_gb": 8,
      "gpu": "integrated",
      "source": "purchase",
      "confidence": 0.8
    }
  },
  "affinities": [
    {"type": "category", "id": "office", "score": 0.9},
    {"type": "task", "id": "documents", "score": 0.8},
    {"type": "category", "id": "security", "score": 0.5}
  ],
  "needs": [
    {"tag": "documents", "score": 0.8},
    {"tag": "data_protection", "score": 0.4}
  ],
  "lifecycle": "renewal_due",
  "price_sensitivity": "medium"
}
```

Что куплено:

| Продукт | Цена | Срок | Места |
|---|---:|---|---:|
| Office Suite | 35 000 | через 30 дней | 10 |
| PDF Toolkit | 10 000 | через 30 дней | 10 |
| Antivirus | 15 000 | через 6 месяцев | 10 |

Событие: клиент открыл лицензию Office.

```json
{
  "client_id": "smb-001",
  "event": "license_view",
  "context": {"license_id": "lic-office-001"},
  "limit": 3
}
```

## 2. Каталог и связи

| ID | Продукт | Цена | Категория | Задача |
|---|---|---:|---|---|
| `prod-office` | Office Suite | 35 000 | office | documents |
| `prod-pdf` | PDF Toolkit | 10 000 | office | documents |
| `prod-sign` | E-signature | 12 000 | office | signature |
| `prod-pack` | Office Pack: Office + PDF + E-sign | 50 000 | office | documents |
| `prod-backup` | Backup | 18 000 | backup | data_protection |
| `prod-antivirus` | Antivirus | 15 000 | security | endpoint_security |
| `prod-game` | Casual Game Pack | 3 000 | games | gaming |
| `prod-raster` | Raster Editor | 22 000 | graphics | raster |
| `prod-vector` | Vector Editor | 20 000 | graphics | vector |
| `prod-video` | Video Editor | 25 000 | graphics | video |

Связи:

| От | К | Тип | Статус |
|---|---|---|---|
| Office | PDF | `adjacent_task` | `confirmed` |
| Office | E-sign | `workflow_next_step` | `confirmed` |
| Office + PDF + E-sign | Office Pack | `bundle` | `confirmed` |
| Antivirus | Backup | `adjacent_task` | `llm_suggested` |
| Raster | Vector | `adjacent_task` | `confirmed` |
| Raster | Raster (другой) | `substitute` | `confirmed` |
| Photo | Video | `workflow_next_step` | `confirmed` |
| Office | Casual Game | `adjacent_task` | `quarantine` |

## 3. Кандидаты

| Кандидат | Тип | Откуда | Правила | Score | Итог |
|---|---|---|---:|---:|---|
| Office Pack | `bundle` / upsell | Office + PDF уже есть, E-sign не хватает | разрешён | 0.89 | 1 |
| Office renewal | `renewal` | лицензия через 30 дней | разрешён | 0.87 | 2 |
| E-signature | `cross_sell` | связь Office → E-sign | разрешён | 0.63 | 3 |
| Backup | `cross_sell` | Antivirus → Backup | разрешён, низкая релевантность | 0.45 | — |
| Casual Game | `cross_sell` | lookalike + environment fit | ниже порога, карантин | 0.24 | — |

## 4. Пакет: математика

```text
current     = 35 000 (Office) + 10 000 (PDF) = 45 000
separate    = 35 000 + 10 000 + 12 000 = 57 000
pack        = 50 000
incremental = 50 000 − 45 000 = 5 000
advantage   = 57 000 − 50 000 = 7 000
```

Пакет дороже текущего чека на 5 000, но E-sign обходится в 5 000 вместо 12 000. Это апсейл, не экономия.

## 5. Ответ

```json
{
  "trace_id": "tr-case-001",
  "client_id": "smb-001",
  "intent": "renewal",
  "recommendations": [
    {
      "product_id": "prod-pack",
      "type": "bundle",
      "goal": "upsell",
      "score": 0.89,
      "rank": 1,
      "reason": "Вы платите 45 000 за Office и PDF. Пакет с E-signature стоит 50 000: E-sign обходится в 5 000 вместо 12 000. Чек +5 000, выгода 7 000.",
      "evidence": [
        {"source": "license", "value": "prod-office, end 2026-12-20"},
        {"source": "license", "value": "prod-pdf, end 2026-12-20"},
        {"source": "catalog", "value": "prod-pack, 50 000"},
        {"source": "catalog", "value": "prod-sign, 12 000"}
      ],
      "constraints": ["переход при продлении"],
      "price": {"amount": 50000, "currency": "RUB"},
      "expires_at": "2026-12-20"
    },
    {
      "product_id": "prod-office",
      "type": "renewal",
      "goal": "retention",
      "score": 0.87,
      "rank": 2,
      "reason": "Лицензия Office истекает через 30 дней. Продление сохраняет 10 мест и текущие условия.",
      "evidence": [
        {"source": "license", "value": "prod-office, end 2026-12-20, seats 10"}
      ],
      "constraints": [],
      "price": {"amount": 35000, "currency": "RUB"},
      "expires_at": "2026-12-20"
    },
    {
      "product_id": "prod-sign",
      "type": "cross_sell",
      "goal": "upsell",
      "score": 0.63,
      "rank": 3,
      "reason": "Вы работаете с документами. E-signature закрывает подпись без печати и сканов.",
      "evidence": [
        {"source": "relation", "value": "Office → E-sign, workflow_next_step"},
        {"source": "profile", "value": "documents 0.8"}
      ],
      "constraints": [],
      "price": {"amount": 12000, "currency": "RUB"},
      "expires_at": "2026-12-20"
    }
  ],
  "fallback": false
}
```

## 6. Что не показали

| Кандидат | Почему |
|---|---|
| Casual Game | ниже порога, связь в карантине |
| Backup | релевантность низкая, не в топ-3 |
| Второй Office | уже куплен |
| Векторный редактор | другая задача, нет сигнала |

## 7. Второй кейс: растр → вектор

Клиент владеет Raster Editor. Открыл каталог графики.

Кандидаты:

| Кандидат | Тип | Решение |
|---|---|---|
| Vector Editor | `adjacent_task` | показываем |
| Другой Raster Editor | `substitute` | не показываем как кросс-сейл |
| Video Editor | `workflow_next_step` | показываем, если есть сигнал видео |

Ответ:

> «У вас растровый редактор. Векторный дополняет его для логотипов и иконок, а не заменяет. Второй растровый редактор — это замена, её в допродажу не берём.»

## 8. Что доказывает кейс

- профиль собирается из фактов, а не из фантазий;
- событие задаёт момент, история — контекст;
- связи товаров проходят через правила и карантин;
- пакет считается как апсейл, а не как скидка;
- заменитель не попадает в кросс-сейл;
- в ответе есть причина и evidence.
