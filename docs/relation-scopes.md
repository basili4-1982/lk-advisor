# Скоупы связей и TTL

Связь живёт не «вообще», а в конкретном кейсе. Одна и та же пара может
работать в B2B/IT и не работать в B2C.

```text
from_product → relation_type → to_product
+ audience: b2b / b2c
+ industry: it / manufacturing / retail / ...
```

## Как выбирается статус

При выдаче рекомендаций для клиента с `audience` и `industry`:

1. Точный скоуп `(audience, industry)`.
2. Скоуп аудитории `(audience, '')`.
3. Скоуп отрасли `('', industry)`.
4. Глобальный статус `product_relations.status`.

Если найденный скоуп `rejected`, `quarantine` или истёк по TTL — связь не
показывается, даже если глобально она `confirmed`.

## Жизненный цикл

```text
draft → testing → confirmed / rejected → expired → re-test
```

| Статус | Сколько живёт | Поведение |
|---|---|---|
| `testing` | до решения | показываем, собираем показы/клики |
| `confirmed` | 30 дней B2B, 60 дней B2C | показываем, следим за guardrails |
| `rejected` | 90 дней до re-test | не показываем |
| `quarantine` | бессрочно до решения | не показываем |

TTL хранится в `relation_scopes.valid_until`. После истечения связь исчезает
из выдачи и помечается как `expired` в админке — это сигнал на re-test.

## Решение по скоупу

Бандит работает по строкам `(from, to, type, audience, industry)`:

```bash
curl -X POST localhost:8080/v1/admin/relations/auto-review \
  -H 'Content-Type: application/json' \
  -d '{"min_shown":20,"min_ctr":0.03}'
```

Правила:

- трогаем только `llm_suggested` и `quarantine`;
- CTR ≥ порога или есть покупки → `confirmed` в этом скоупе;
- 0 кликов после `min_shown` → `rejected` в этом скоупе;
- глобальный `product_relations.status` не меняется;
- семантику подтверждает человек.

## Человеческое решение

```bash
curl -X POST localhost:8080/v1/admin/relations/scopes/review \
  -H 'Content-Type: application/json' \
  -d '{
    "from_id":"prod-lk-adobe",
    "to_id":"prod-sl-43",
    "type":"alternative",
    "audience":"b2b",
    "industry":"it",
    "decision":"confirm",
    "ttl_days":30,
    "note":"проверено на пилоте"
  }'
```

`decision`: `confirm`, `reject`, `quarantine`, `reopen`.

## API

```text
GET  /v1/admin/relations/scopes
POST /v1/admin/relations/scopes/review
GET  /v1/admin/relations/performance
POST /v1/admin/relations/auto-review
```

`performance` теперь возвращает разрез:

```json
{
  "from_id": "prod-pdf",
  "to_id": "prod-backup",
  "audience": "b2b",
  "industry": "it",
  "status": "confirmed",
  "valid_until": "2026-12-26T12:00:00Z",
  "shown": 120,
  "clicks": 14,
  "purchases": 3
}
```

## Админка

`/admin#analytics`:

- «Работающие связи» — показы, клики, CTR, покупки, скоуп, статус, TTL;
- «Скоупы связей и TTL» — подтвердить, отклонить или вернуть в тест;
- «Авто-решение бандита» — решения по каждому скоупу.

## Пример

Связь `Office → Антивирус`:

| Скоуп | Показы | Клики | Решение |
|---|---:|---:|---|
| B2B, manufacturing | 40 | 8 | `confirmed`, 30 дней |
| B2B, IT | 25 | 1 | остаётся `testing` |
| B2C | 30 | 0 | `rejected`, re-test через 90 дней |

Раньше это смешалось бы в один CTR и одно глобальное решение. Теперь у
каждого кейса свой статус и свой TTL.
