# Relation-level бандит: какая связь реально работает

Цель: проверять не товар и не стратегию вообще, а конкретную связь:

```text
from_product → relation_type → to_product
```

## Что добавлено

- `candidate_id` в показах, кликах и покупках;
- `source_product_id` и `relation_type` в событиях;
- `GET /v1/admin/relations/performance` — CTR/покупки по каждой связи;
- `POST /v1/admin/relations/auto-review` — продвинуть или отклонить связь;
- таблица «Работающие связи» в `/admin#analytics`.

## Как считается

```text
candidate_id = from_id + "|" + relation_type + "|" + to_id
```

Пример:

```text
prod-lk-adobe|alternative|prod-sl-43
```

Для этой связи считаем:

- показы;
- клики;
- покупки;
- CTR;
- conversion;
- статус связи.

Клик наследует `candidate_id`, `source_product_id`, `relation_type` из показа. Поэтому можно понять, какая именно связь дала результат.

## Авто-решение

`POST /v1/admin/relations/auto-review`

```json
{"min_shown": 20, "min_ctr": 0.03}
```

Правила:

- трогаем только `llm_suggested` и `quarantine`;
- если CTR ≥ `min_ctr` или есть покупки → `confirmed`;
- если 0 кликов после `min_shown` → `rejected`;
- всё пишется в аудит;
- семантически спорные связи всё равно проверяет человек.

## Что уже видно

Для `lk-36612`:

```text
Adobe renewal          → prod-lk-adobe|renewal|prod-lk-adobe
Microsoft 365 Business → prod-lk-m365|upgrade|prod-office-business
CorelDRAW alternative  → prod-lk-adobe|alternative|prod-sl-43
```

То есть теперь можно сравнить не «стратегию вообще», а:

- renewal Adobe;
- upgrade на Business;
- alternative на Corel.

## Ограничения

- Пока это работает для `product_relations`; кандидаты из `lk_relation_candidates` ещё не подключены к выдаче.
- Для B2C мало трафика: 20–30 показов на связь может копиться долго.
- CTR не равен марже; для маржи нужна цена/себестоимость.
- Auto-review не меняет semantic correctness — только эффективность.
- Нужна защита от position bias: сейчас rank сохраняется, но CTR по rank ещё не нормализуется.

## Следующий шаг

1. Подключить `lk_relation_candidates` к рекомендательной выдаче как экспериментальные связи.
2. Считать posterior по связи, а не только сырой CTR.
3. Добавить promotion только после минимальной выборки и проверки guardrails.
4. Сравнить «связь работает» vs «связь не работает» на когорте агентов.
