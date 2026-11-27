# API и пересчёт рекомендаций

Публичная спецификация: [`docs/openapi.yaml`](openapi.yaml).

Покрыто:

- `POST /v1/recommendations` — top-3, `certified_only`, политика отраслей;
- `POST /v1/events` — клик/покупка по `trace_id`;
- `POST /v1/ask` — агент с инструментами и памятью;
- `GET /v1/clients/{id}/profile`;
- `GET /v1/products`;
- `GET /v1/agent/demo`;
- `GET /v1/lk/clients` — демо-витрина.

Админские ручки `/v1/admin/*` — внутренний контур, в публичную спеку не входят.

## Пересчёт рекомендаций

```bash
go run ./cmd/lk-advisor recompute
```

Считает top-10 офлайн-моделью и заменяет `client_recommendations` одной
транзакцией.

Планировщик в `serve`:

| Переменная | Значение |
|---|---|
| `RECOMPUTE_INTERVAL` | `24h` по умолчанию, `0`/`off` — выключить |
| `RECOMPUTE_ON_START` | `1` — пересчитать при старте |
| `RECOMPUTE_TOP` | сколько рекомендаций хранить, по умолчанию 10 |

## Политика проверки связей

Порог автоприёма и порог отправки в карантин настраиваются:

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `RELATION_AUTO_CONFIRM` | `true` | автоприём связей выше порога |
| `RELATION_AUTO_CONFIRM_THRESHOLD` | `0.85` | от этой уверенности связь подтверждается без человека |
| `RELATION_REVIEW_THRESHOLD` | `0.70` | ниже этого порога связь уходит в карантин |

Диапазоны: `>= 0.85` — `confirmed`; `0.70–0.85` — `llm_suggested`, кандидат
на проверку человеку; `< 0.70` — `quarantine`. Если выставить
`RELATION_AUTO_CONFIRM=false`, через человека проходят все связи.

## Пример

```bash
curl -X POST localhost:8080/v1/recommendations \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"C05445","limit":3}'
```
