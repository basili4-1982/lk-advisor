# Платформа проверки гипотез: вертикальный срез

Одна гипотеза проходит полный цикл:

```text
гипотеза → назначение → exposure → метрики → решение
```

Это не «ещё один A/B», а слой, который связывает рекомендацию с проверяемым
эффектом и решением.

## Демо-гипотеза

```text
PDF Toolkit как следующий шаг после Office
Показ PDF Toolkit в слоте 3 повышает конверсию в B2B
```

- скоуп: `b2b`;
- варианты: `control` / `treatment`, 50/50;
- treatment принудительно ставит `prod-pdf` в оффер;
- метрика: conversion;
- guardrails: не терять renewal и upsell, следить за fallback.

## Как это работает

1. Клиент детерминированно попадает в `control` или `treatment` по хэшу
   `experiment_id + client_id`.
2. Каждый запрос рекомендаций пишет `exposed` в `experiment_events`.
3. Если target показан, exposure привязывается к `product_id`.
4. Клик и покупка target копируются в `experiment_events`.
5. Результаты считаются по вариантам, включая:
   - exposures;
   - target shown;
   - clicks;
   - purchases;
   - CTR;
   - overall conversion;
   - target conversion.
6. Считаются guardrails: renewal, upsell, fallback.
7. Человек принимает решение: `promote`, `reject`, `extend`.

## Почему target conversion важнее uplift

Синтетический прогон 50 клиентов × 10 сессий:

| Вариант | Exposure | Target shown | Покупки | Conversion | Target conv |
|---|---:|---:|---:|---:|---:|
| control | 250 | 29 | 8 | 3.2% | **27.6%** |
| treatment | 250 | 231 | 65 | 26.0% | **28.1%** |

Guardrails в том же прогоне:

| Вариант | Renewal | Upsell | Fallback |
|---|---:|---:|---:|
| control | 76.0% | 20.0% | 0.0% |
| treatment | 72.0% | 24.0% | 0.0% |

Renewal слегка падает. Поэтому решение — не `promote`, а `extend`: нужна
вторая волна и проверка, не теряем ли продления.

Общий uplift выглядит огромным, но target conversion одинаковый.

Вывод: **эффект не в том, что товар стал лучше, а в том, что его перестали
прятать**. Если бы target conversion упал, гипотеза была бы плохой, даже при
росте общего conversion.

Это и есть ценность платформы: она показывает, **где именно** возникает эффект.

## API

```text
GET  /v1/admin/experiments
POST /v1/admin/experiments/demo
POST /v1/admin/experiments/{id}/decision
```

Решение:

```json
{"decision":"promote","note":"подтверждено на пилоте"}
```

## Схема

- `experiments` — гипотеза, скоуп, варианты, target, статус, решение;
- `experiment_assignments` — стабильный вариант клиента;
- `experiment_events` — exposed / clicked / purchased + guardrail-поля.

`extend` не просто меняет статус: эксперимент остаётся running и получает
`ends_at = now() + 14 дней` для следующей волны.

## База знаний

Завершённые эксперименты не удаляются: в таблице `experiments` остаются
гипотеза, скоуп, метрики, решение и заметка. Это и есть первая версия базы
знаний: что проверяли, где, с каким результатом и что решили.

## Ограничения

- Это синтетическая симуляция, не рыночное доказательство.
- 25 клиентов на вариант — мало для статистической значимости.
- Нет маржи: нужна себестоимость.
- Нет guardrail-метрик по renewal/upsell в этой версии.
- Решение принимает человек; платформа не меняет семантику связи сама.

## Следующий шаг

- guardrails: renewal, upsell, fallback, margin;
- sequential testing и минимальная выборка;
- база знаний: гипотеза → результат → скоуп → TTL;
- пилот на реальном трафике ЛК.
