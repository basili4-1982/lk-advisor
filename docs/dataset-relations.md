# Досвязывание реального датасета v3

Первый вертикальный срез: **импорт → co-purchase → кандидаты → очередь ревью**.

## Что импортировано

Источник: `Датасет_Рекомендации_ЛК_v3.xlsx`. Файл в репозиторий не копируется.

| Сущность | Количество |
|---|---:|
| Товары (`lk_items`) | 1 079 |
| Клиенты (`lk_clients`) | 19 162 |
| Заказы (`lk_orders`) | 27 878 |
| Строки продаж | 30 892 |
| Co-purchase пары | 2 410 |
| Пары с count ≥ 2 | 735 |
| Товары в парах | 735 |
| Solo-товары | 344 |

## Как устроено

```text
Sales XLSX
  → lk_items / lk_clients / lk_orders
  → co_purchase_pairs (count, B2B, B2C)
  → конструктор связей: активные шаблоны + LLM
  → lk_relation_candidates (quarantine)
  → ручное подтверждение / отклонение
```

Co-purchase — это **кандидат и evidence**, а не готовый тип связи. Тип выбирает конструктор:

- активные `relation_types`;
- описание, примеры и поведение;
- LLM-классификация;
- fallback по названиям: `upgrade`, `accessory`, `adjacent_task`.

## API

```bash
GET  /v1/admin/dataset/coverage
GET  /v1/admin/dataset/co-purchases?limit=50
POST /v1/admin/dataset/co-purchases/suggest      # быстрый adjacent_task
POST /v1/admin/dataset/co-purchases/classify     # конструктор + LLM
GET  /v1/admin/dataset/candidates?limit=100
POST /v1/admin/dataset/candidates/review
```

## Админка

`/admin#dataset`:

- карточки: товары, клиенты, заказы, пары, пары ≥2, solo, кандидаты;
- таблица топ-пар;
- кнопка «Классифицировать топ-10 через конструктор»;
- таблица кандидатов с типом, confidence, статусом, причиной;
- подтверждение / отклонение.

## Что уже видно на реальных данных

- 2 410 пар; 735 пар с count ≥ 2.
- 344 solo-товара — с ними нужна отдельная стратегия: кейс, сервис, открытые источники.
- Сильные пары — это в основном лицензии/дистрибутивы/тарифы:
  - Google Workspace Business ↔ Starter;
  - КриптоПро дистрибутив ↔ лицензия;
  - МагПро дистрибутив ↔ лицензия.
- Конструктор уже даёт `accessory` для дистрибутив/лицензия и `upgrade` для Starter → Business.

## Ограничения

- Категории товаров определены эвристикой по названию; фид ещё не подключён.
- Нет маппинга `item_id` → `product_id` реального каталога.
- Нет batch-классификации на все 735 пар; сейчас топ-10.
- Co-purchase не работает для B2C (0.8% мультитоварных заказов).
- Solo-товары пока только считаются; стратегия не реализована.
- LLM-классификация идёт последовательно; для сотен пар нужна очередь/воркеры.

## Следующий шаг

1. Batch-классификация 100–735 пар.
2. Отдельная стратегия для 344 solo: атрибуты, кейсы, открытые источники.
3. Маппинг в реальный каталог и `product_relations`.
4. Coverage-дашборд по категориям.
5. Подключить подтверждённые связи к рекомендателю.

## Воспроизведение

```bash
DATABASE_URL='postgres://lk:lk@localhost:55432/lk?sslmode=disable' \
  go run ./cmd/lk-advisor migrate
python3 analysis/import_dataset.py
```

Классификация:

```bash
curl -s -X POST localhost:8080/v1/admin/dataset/co-purchases/classify \
  -H 'Content-Type: application/json' -d '{"limit":10}'
```
