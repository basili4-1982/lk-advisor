# Демо: один прогон

## Подготовка

```bash
make db-up       # PostgreSQL 16 на :55432
make migrate
make seed        # seed 42: 50 клиентов, 200 товаров, 40 акций
make run         # сервис на :8080
```

Открыть `http://localhost:8080/` — демо-кабинет. Жюри ничего нажимать не нужно: профиль, лицензии и рекомендация уже на экране, ответ ассистента загружается автоматически.

LLM берёт ключ из `DEEPSEEK_API_KEY` (или `LLM_API_KEY`). Без ключа агент отвечает шаблоном.

## Настройка правил

Открыть `http://localhost:8080/admin` — связи товаров, ограничения, веса и ИИ-помощник. Подробности: `docs/admin-rules.md`.

## Прогон

```bash
BASE=http://localhost:8080 ./docs/demo.sh
```

Шесть сценариев:

1. `lk-36612` — реальный кейс тестового ЛК: Adobe истёк, Kaspersky/Dr.Web активны, Microsoft 365 Personal.
2. `smb-001` — renewal: лицензия Antivirus истекает через 14 дней.
3. `smb-002` — cross-sell: Vector Editor, активная акция 10% до 30.11.
4. `smb-005` — upsell: Vector Editor Pro с `manager_approval`.
5. `/v1/ask` «Почему векторный?» — 2 инструмента, ответ по evidence.
6. «Какая погода?» — отказ без вызова инструментов.
7. `smb-006` — fallback: популярное в сегменте.

## Что смотреть в JSON

- `intent`, `recommendations[0].product_id`;
- `reason` и `evidence` — числа только из данных;
- `promo` — скидка и срок;
- `constraints` — `manager_approval`;
- `tool_calls` у `/v1/ask` — какие инструменты вызваны;
- `fallback` — признак запасного сценария.

## Резерв

Если API не отвечает, показать сохранённые ответы:

```bash
curl -sS http://localhost:8080/healthz
```

Резервный JSON — в `docs/demo-run.md` после первого прогона (вставить фактические ответы).

## Сброс данных

```bash
DATABASE_URL='postgres://lk:lk@localhost:55432/lk?sslmode=disable' \
  go run ./cmd/lk-advisor seed --seed 42
```

Seed идемпотентен: очищает таблицы и загружает заново.
