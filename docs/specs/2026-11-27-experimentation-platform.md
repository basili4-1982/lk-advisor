# Платформа экспериментов и подтверждения эффекта

## 1. Зачем это

Сейчас рекомендательная система отвечает на вопрос «что показать». Заказчику нужен ответ на другой вопрос: **«что сработает и сколько это принесёт»**.

Продукт должен позволять:

1. Сформулировать гипотезу.
2. Настроить эксперимент: аудитория, вариант, метрика, guardrails.
3. Показать вариант одной части клиентов и контроль — другой.
4. Собрать показы, клики, покупки, выручку и маржу.
5. Посчитать uplift и доверительный интервал.
6. Принять решение: внедрить, откатить или продлить.
7. Сохранить результат в базу знаний и обновить рекомендации.

То есть рекомендатель становится **генератором вариантов**, а платформа — **судьёй, который доказывает эффект**.

## 2. Кто пользователь

| Роль | Что делает |
|---|---|
| Product / merchandiser | создаёт гипотезу, выбирает аудиторию, читает результат |
| Analyst | задаёт метрики, размер выборки, проверяет валидность |
| Engineer | подключает эксперимент к выдаче, следит за ошибками |
| Агент | предлагает гипотезы, считает sample size, следит за guardrails, готовит решение |
| Заказчик | видит доказанный uplift и принимает бизнес-решение |

## 3. Главный объект — эксперимент

```text
Гипотеза
  → аудитория
  → варианты (control / treatment)
  → метрика успеха
  → guardrails
  → размер выборки и срок
  → запуск
  → анализ
  → решение
  → база знаний
```

Эксперимент — не «включить фичу». Это контракт:

```json
{
  "hypothesis": "Антивирус как дополнение к Office увеличит attachment rate на 10%",
  "audience": "B2B, есть Office, нет антивируса",
  "unit": "client_id",
  "variants": ["control", "treatment"],
  "primary_metric": "attachment_rate_30d",
  "guardrails": ["margin_floor", "no_second_security_product"],
  "min_sample_per_arm": 3000,
  "duration_days": 28
}
```

## 4. Типы экспериментов

| Тип | Что меняем | Пример |
|---|---|---|
| **Relation injection** | добавляем связь/кандидат | антивирус к Office |
| **Promo / pricing** | акция, срок, скидка | 1+1 антивирус на 2 года |
| **Ranking / weights** | веса или стратегия | hybrid vs popular |
| **Cold start** | fallback/стартовый набор | отрасль vs популярность |
| **Solo / service** | кейс/сервис вместо товара | шифрование → уничтожение файлов |
| **Message / reason** | формулировка объяснения | «часто покупают вместе» vs «популярно» |

## 5. Пример 1: антивирус как дополнение к Office

**Гипотеза:** если владельцу Office показать антивирус в топ-3, attachment rate вырастет на 10% относительно контроля.

- Аудитория: B2B, есть Office, нет активного антивируса.
- Контроль: текущий топ-3.
- Тест: в топ-3 добавляется антивирус.
- Основная метрика: покупка антивируса в течение 30 дней / клиент.
- Дополнительно: выручка, маржа, продление Office.
- Guardrails: не показывать второй security-продукт, не ниже margin floor.
- Решение: ship, если uplift ≥ 10% и нижняя граница CI > 0, guardrails не нарушены.

## 6. Пример 2: 1+1 антивирус на 2 года

**Гипотеза:** акция «1+1 на 2 года» увеличит маржу на eligible-клиента, а не просто выручку.

- Аудитория: покупатели/продления антивируса.
- Контроль: обычная цена.
- Тест: 1+1 на 2 года.
- Основная метрика: **маржа на eligible-клиента**.
- Дополнительно: take-up, renewal, cannibalization, refund rate.
- Guardrails: margin floor, max discount, отсутствие конфликта каналов.
- Решение: ship, если маржа выше контроля с учётом каннибализации.

**Критично:** для маржи нужна себестоимость. В датасете есть цены, но нет cost. Если cost нет, эксперимент может мерить только revenue/take-up, а margin надо либо получить от заказчика, либо считать моделью. Это открытый вопрос.

## 7. Метрики

### Основные

- conversion / attachment rate;
- revenue per eligible client;
- **gross margin per eligible client**;
- renewal / retention;
- take-up промо.

### Вспомогательные

- CTR, показы, клики;
- AOV;
- доля каннибализации;
- refund/cancel rate;
- Fill Rate.

### Guardrails

- margin floor;
- max discount;
- запрет второго security-продукта;
- запрет товаров из стоп-листа;
- рост возвратов/жалоб.

## 8. Рандомизация

- Единица: `client_id` (для B2B — компания; для B2C — клиент).
- Sticky assignment: `hash(experiment_id + client_id) % 100`.
- Eligibility-фильтры до назначения.
- Один клиент — только один вариант.
- Holdout-группа для долгосрочного эффекта.
- Экспозиция пишется только когда вариант реально показан.

## 9. Анализ

- Доли: two-proportion z-test + Bayesian Beta-Binomial.
- Выручка/маржа: t-test или bootstrap.
- Sequential testing / always-valid p-values, чтобы не ломать тест подглядыванием.
- SRM-проверка: доли вариантов не должны разъехаться.
- CUPED по pre-period, если есть история.
- Сегменты: B2B/B2C, отрасль, студент, cold start; поправка на множественные сравнения.
- Novelty/primacy: минимум 2 недели, сравнение по неделям.
- Решение: ship / rollback / extend / inconclusive.

## 10. Агентный слой

Агент:

1. Читает аналитику: низкий CTR, высокий skip, высокая маржа, просадка.
2. Предлагает гипотезу и вариант.
3. Считает sample size и срок.
4. Проверяет guardrails.
5. Запускает только после подтверждения человеком.
6. Следит за SRM и guardrails.
7. Готовит решение с evidence.
8. После подтверждения обновляет связь/вес/промо и базу знаний.

Агент **не** меняет семантику связей и запреты без человека.

## 11. Данные

```text
experiments
experiment_variants
experiment_assignments
experiment_exposures
experiment_outcomes
experiment_results
experiment_decisions
experiment_knowledge
```

Ключевые поля:

- `experiments`: hypothesis, audience, unit, primary_metric, guardrails, status, start/end.
- `variants`: control/treatment, allocation, config_patch.
- `assignments`: experiment_id, unit_id, variant, hash, assigned_at.
- `exposures`: experiment_id, unit_id, variant, trace_id, shown_at.
- `outcomes`: purchase/revenue/margin/refund, ts.
- `results`: uplift, ci, p_value/bayes, srm, segments, decision.

## 12. API

```text
POST   /v1/experiments
GET    /v1/experiments
GET    /v1/experiments/{id}
POST   /v1/experiments/{id}/start
POST   /v1/experiments/{id}/stop
GET    /v1/experiments/{id}/results
POST   /v1/experiments/{id}/decide
POST   /v1/experiments/{id}/apply
POST   /v1/experiments/{id}/agent/propose
```

Рекомендательная ручка получает assignment и возвращает `experiment_id` + `variant`, чтобы клиент/логгер могли связать показ с экспериментом.

## 13. UI

- Список экспериментов и статусы.
- Конструктор: гипотеза, аудитория, вариант, метрика, guardrails.
- Калькулятор sample size.
- Монитор: SRM, показы, конверсия, CI, guardrails.
- Результаты: uplift, сегменты, решение.
- База знаний: что проверяли, что вышло, что внедрили.

## 14. Не входит

- Полноценный feature-flag движок.
- Замена Optimizely/Amplitude.
- Мультитенантность.
- Автоматическое изменение семантики связей.
- Реальные интеграции без данных заказчика.

## 15. MVP

1. Один тип эксперимента: **relation injection** + **promo**.
2. Sticky assignment и экспозиция.
3. Две метрики: conversion/attachment и revenue; margin — если есть cost.
4. Two-proportion + Bayesian.
5. SRM и guardrails.
6. Дашборд и решение.
7. База знаний и ручное применение.

## 16. Открытые вопросы к заказчику

1. **Есть ли себестоимость товаров?** Без неё нельзя честно мерить маржу.
2. Единица рандомизации: клиент, компания, пользователь, заказ?
3. Сколько трафика в день на B2B и B2C?
4. Какие поверхности экспериментируем: главная ЛК, каталог, email, менеджер?
5. Кто принимает решение ship/rollback: человек или агент?
6. Какие guardrails обязательны: margin floor, max discount, стоп-листы?
7. Сколько недель заказчик готов ждать результат?
