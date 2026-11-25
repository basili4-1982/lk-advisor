# Модель данных и синтетика

## 1. Сущности

| Сущность | Поля | Зачем |
|---|---|---|
| `Client` | id, name, segment, industry, size, region | сегмент и контекст |
| `Product` | id, sku, title, category, subcategory, vendor, type, price, term_months, seats, version, status, system_requirements | каталог |
| `ClientEnvironment` | client_id, os, ram_gb, cpu, gpu, source, confidence | что запустится у клиента |
| `ProductRelation` | from_id, to_id, type, direction, confidence, source | связи товаров |
| `Purchase` | id, client_id, product_id, date, amount, quantity, seats | история |
| `License` | id, client_id, product_id, seats, start, end, status | продление и апгрейд |
| `Order` | id, client_id, status, created_at, delivered_at, items | причина визита |
| `Document` | id, client_id, order_id, type, created_at, downloaded_at | документы |
| `Event` | id, client_id, type, ts, context | вход, просмотр, скачивание |
| `Rule` | id, type, condition, action | запреты и ограничения |
| `GroundTruth` | client_id, event, expected_product_id, expected_type | оценка |

### Типы связей

`compatibility`, `upgrade`, `accessory`, `analog`, `bundle`, `requires`, `excludes`.

Направление: `from → to`. Для апгрейда `from` — младший, `to` — старший.

### Типы правил

| Тип | Пример | Действие |
|---|---|---|
| `excludes` | лицензия A несовместима с B | исключить |
| `manager_only` | апгрейд только через менеджера | пометить `manager_approval` |
| `export_control` | товар под экспортным контролем | исключить |
| `already_owned` | товар уже куплен | исключить, кроме апгрейда |
| `out_of_stock` | нет в наличии | исключить или пометить |
| `max_discount` | скидка не выше 10% | ограничить |

## 1.1. Профиль клиента

Профиль — отдельная сущность, не таблица в каталоге. Он собирается из фактов и таксономии.

```json
{
  "client_id": "smb-001",
  "schema_version": "1.0",
  "updated_at": "2026-11-19T15:00:00Z",
  "facts": {
    "segment": "SMB",
    "industry": "e-commerce",
    "size": "50-200",
    "tenure_months": 18,
    "active_licenses": 2,
    "renewals_next_90d": 1,
    "events_30d": 12,
    "last_order": {"order_id": "ord-123", "status": "delivered", "date": "2026-11-18"}
  },
  "affinities": [
    {"type": "category", "id": "graphics", "score": 0.7, "evidence": [{"source": "purchase", "value": "prod-raster"}]},
    {"type": "task", "id": "raster", "score": 0.6, "evidence": [{"source": "catalog", "value": "task_tags"}]}
  ],
  "needs": [
    {"tag": "content", "score": 0.7, "evidence": [{"source": "event", "value": "document_download guide-video"}]}
  ],
  "lifecycle": "renewal_due",
  "price_sensitivity": "medium",
  "summary": "SMB e-commerce, две лицензии, продление через 14 дней, активен в контенте."
}
```

Как собираем:

1. Факты из покупок, лицензий, заказов и событий.
2. Аффинити по категориям и задачам: покупки, суммы, давность, частота.
3. Вес: свежие покупки весят больше, старые затухают.
4. Теги потребностей: из таксономии, с evidence.
5. LLM добавляет `summary` и качественные теги, но только с evidence.
6. Валидатор: схема, словарь, evidence.
7. Кэш на 7 дней или до новой покупки.

Профиль не выбирает товар. Он сужает кандидатов и даёт аргументы.

### Ресурсная совместимость

Отдельный слой: что у клиента уже работает и что на этом запустится.

```json
{
  "client_id": "smb-001",
  "environment": {
    "os": "Windows 11",
    "ram_gb": 8,
    "cpu": "4 ядра",
    "gpu": "integrated",
    "source": "purchase",
    "confidence": 0.8
  }
}
```

У товара:

```json
{
  "product_id": "prod-game-casual",
  "system_requirements": {
    "os": "Windows 10+",
    "ram_gb": 4,
    "cpu": "2 ядра",
    "gpu": "integrated",
    "disk_gb": 8
  }
}
```

Правило: `requirements <= environment`. Если игра требует 4 ГБ, а у клиента 8 — совместимо. Если 16 — нет.

Это открывает кросс-категорийные рекомендации: офисный пакет и казуальная игра могут требовать одно и то же железо.

## 2. Синтетический датасет

Объём:

- 50 SMB-клиентов;
- 200 товаров в 10 категориях;
- 300 связей товаров;
- 500 покупок;
- 100 лицензий;
- 200 заказов;
- 1000 событий;
- 20 запретов и ограничений;
- 50 размеченных кейсов.

Файлы:

```text
data/
  clients.csv
  products.csv
  relations.csv
  purchases.csv
  licenses.csv
  orders.csv
  events.csv
  rules.csv
  ground_truth.csv
```

Seed: 42. Все суммы в рублях, даты в ISO 8601.

## 3. Распределения

- 60% клиентов — 1–3 покупки, 30% — 4–10, 10% — больше 10.
- 40% лицензий истекают в ближайшие 90 дней.
- 30% заказов доставлены в последние 30 дней.
- 20% клиентов без истории — для fallback.
- 10% товаров — аналоги снятых с продажи.
- 5% связей — неподтверждённые, с `confidence < 0.7`.

## 4. Сценарии для демо

| # | Сценарий | Вход | Ожидание |
|---|---|---|---|
| 1 | Продление | `license_view`, лицензия истекает через 14 дней | `renewal`, продление или апгрейд |
| 2 | Заказ | `order_view`, заказ доставлен вчера | `cross_sell`, аксессуар или гарантия |
| 3 | Апгрейд | клиент у лимита мест | `upsell`, старший тариф |
| 4 | Запрет | несовместимый товар | товара нет в ответе |
| 5 | Менеджер | апгрейд только через менеджера | товар есть, `manager_approval` |
| 6 | Нет данных | новый клиент | `fallback: true`, популярное в сегменте |

## 5. Ground truth

50 кейсов вида:

```json
{
  "client_id": "smb-001",
  "event": "license_view",
  "context": {"license_id": "lic-456"},
  "expected_product_id": "prod-101",
  "expected_type": "renewal",
  "acceptable_alternatives": ["prod-102"],
  "notes": "продление на 12 месяцев"
}
```

Метрики считаются по `expected_product_id` и `acceptable_alternatives`.

## 6. Что проверить на данных

- [ ] У каждого клиента есть сегмент и отрасль.
- [ ] У каждой лицензии есть дата окончания.
- [ ] У каждого заказа есть статус и дата доставки.
- [ ] У каждого товара есть категория и вендор.
- [ ] Связи не содержат дублей и обратных апгрейдов.
- [ ] Запреты не противоречат друг другу.
- [ ] В ground truth нет товаров, которых нет в каталоге.
- [ ] 20% клиентов без истории действительно без покупок.
