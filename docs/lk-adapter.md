# Адаптер под `cross_sell/list`

Тестовый ЛК вызывает `POST /api/cross_sell/list` и ждёт:

```json
{
  "status": "shown",
  "data": [
    {"link": "...", "image": "...", "name": "...", "vendorName": "...", "vendorLink": "...", "price": 738.99, "priceWithDiscount": 738.99}
  ]
}
```

Наш сервис отдаёт тот же контракт, но добавляет причину, evidence и score.

## Запрос

```bash
curl -s -X POST localhost:8080/api/cross_sell/list \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"lk-36612"}'
```

Можно передать контекст без записи в БД:

```json
{"client_id":"lk-36612","audience":"b2b","industry":"IT"}
```

## Ответ

```json
{
  "status": "shown",
  "data": [
    {
      "link": "https://store.softline.ru/search/?text=...",
      "image": "",
      "name": "CorelDRAW Graphics Suite 2026",
      "vendorName": "Sarnas",
      "vendorLink": "https://store.softline.ru/search/?text=Sarnas",
      "price": 124470,
      "priceWithDiscount": 124470,
      "product_id": "prod-sl-43",
      "type": "analog",
      "reason": "Замените снятый продукт на CorelDRAW Graphics Suite 2026: замена Adobe Creative Cloud...",
      "evidence": [{"source": "relation", "value": "замена Adobe Creative Cloud..."}],
      "score": 0.95
    }
  ]
}
```

`status=hidden` и пустой `data`, если рекомендаций нет.

## Что заменяется

- `status` — `shown` / `hidden`;
- `data` — до трёх карточек;
- `link` — поиск по названию на store.softline.ru (реальный URL товара появится, когда будет фид);
- `image` — пусто, пока нет картинок;
- `price` / `priceWithDiscount` — из каталога и акции;
- `vendorName` / `vendorLink` — из карточки товара.

Дополнительно: `product_id`, `type`, `reason`, `evidence`, `score` — для проверки и аналитики.

## Реальный кейс

После `seed` в БД появляется клиент `lk-36612`:

- Kaspersky Endpoint Security, 45 мест;
- Adobe Creative Cloud, истёк, доступна замена;
- Dr.Web Desktop Security Suite, 10 мест;
- Microsoft 365 Personal, 1 место.

Ожидаемый топ-3:

1. замена Adobe → CorelDRAW Graphics Suite;
2. курс по Office;
3. PDF Toolkit.

Это ровно то, чего нет в текущем блоке ЛК: он показывает Kaspersky/КриптоПро и игнорирует истёкший Adobe.
