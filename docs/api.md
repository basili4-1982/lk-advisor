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

## Пример

```bash
curl -X POST localhost:8080/v1/recommendations \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"C05445","limit":3}'
```
