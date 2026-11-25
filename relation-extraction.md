# Извлечение связей товаров

## 1. Зачем

Проверить, может ли LLM связать товары без ручных правил: замена, дополнение, совместимость, аксессуар. И где ей нельзя доверять.

## 2. Датасет

- Источник: `luminati-io/eCommerce-dataset-samples`, `amazon-products.csv` — публичный сэмпл Amazon.
- Взяли 438 товаров из 8 категорий: Electronics, Cell Phones, Office, Home & Kitchen, Tools, Appliances, Video Games, Industrial.
- Поля: `id`, `title`, `brand`, `category`, `subcategory`, `description`, `features`, `price`, `currency`.
- Файл: `data/external/amazon-sample-438.csv`.
- 20 пар для теста: `data/external/pairs-sample.jsonl`.
- Результаты LLM: `data/external/relations-llm.jsonl`.
- Ручная проверка: `data/external/relations-review.csv`.

## 3. Промпт

```text
Ты классифицируешь связи между двумя товарами интернет-магазина.
Используй только карточки. Не выдумывай совместимость, модель или характеристики.
Если данных не хватает — relation="none", confidence низкая.
Верни строго JSON:
{"pair_id":"...","relation":"same_task|substitute|adjacent_task|accessory|compatibility|workflow_next_step|bundle|upgrade|none","direction":"a_to_b|b_to_a|both|none","confidence":0.0,"evidence":"цитата или факт","show":true}
show=true только если связь подтверждена и уместна для рекомендации.
```

## 4. Результаты на 20 парах

| Тип | Сколько |
|---|---:|
| `substitute` | 7 |
| `none` | 5 |
| `adjacent_task` | 4 |
| `compatibility` | 2 |
| `accessory` | 1 |
| `same_task` | 1 |

`show=true` — 14 из 20. Для кросс-сейла это слишком много: 7 заменителей попали в `show=true`.

### Что получилось хорошо

- LED-лампа и настольная лампа — `none`, цоколь не подтверждён.
- Амбушюры AirPods и другие наушники — `none`, модель не совпадает.
- Солнечные светильники и GU10 — `none`.
- HDMI-кабель и KVM — `compatibility`, `show=true`.
- Ethernet-коммутатор и кабель — `accessory`, `show=true`.
- Клавиатура и коврик, кресло и коврик — `adjacent_task`, `show=true`.

### Где ошиблась

- Чехлы для разных iPad — `substitute`, но `show=true`. Для кросс-сейла нельзя.
- Чехлы для разных iPhone — то же.
- Коврики, лампы, колонки, льдогенераторы — `substitute`, `show=true`.
- Корзины Whirlpool — `substitute`, `show=true` без подтверждения модели.
- Контейнеры и коврики для холодильника — `adjacent_task` с 0.55 и `show=true`, слабо.

## 5. Вывод

LLM хорошо отвечает «какая это связь» и умеет отказываться. LLM плохо отвечает «показывать ли это клиенту». `show` считает движок, не модель.

## 6. Правила валидатора

```text
cross_sell:  accessory | compatibility | adjacent_task | workflow_next_step
replacement: substitute | same_task — только если у клиента есть та же модель
block:       none | rejected | confidence < 0.7 | нет evidence
compatibility: только при явном подтверждении модели, цоколя, разъёма
```

Дополнительно:

- `substitute` не показываем как допродажу;
- `same_task` — только замена, не кросс-сейл;
- `adjacent_task` с confidence < 0.7 — в карантин;
- `show` из ответа LLM игнорируем.

## 7. Статусы связи

| Статус | Что значит | Показываем |
|---|---|---|
| `confirmed` | подтверждена атрибутами, поведением или экспертом | да |
| `llm_suggested` | предложена LLM, evidence есть, порог пройден | да, если тип разрешён |
| `quarantine` | низкая уверенность или нет evidence | нет |
| `rejected` | конфликт, запрет, заменитель в кросс-сейле | нет |
| `manual_override` | ручное решение аналитика | по решению |

Ручное решение хранится с автором и временем, переопределяет валидатор и попадает в аудит.

## 8. Что дальше

- расширить до 50–100 пар, обязательно с дополняющими товарами;
- считать precision по каждому типу;
- проверить prompt v2 с явным запретом `show` для `substitute`;
- сравнить с правилами без LLM;
- оставить `relations-review.csv` для ручной проверки.

## 9. Prompt v2

Расширили выборку до 60 пар:

- 20 прежних;
- 20 пар внутри категорий;
- 10 пар между соседними категориями;
- 10 случайных пар как контроль.

Разделили два решения:

```json
{
  "show_cross_sell": false,
  "show_replacement": false
}
```

Правила в промпте:

- `show_cross_sell=true` только для `accessory`, `compatibility`, `adjacent_task`, `workflow_next_step`;
- для `substitute`, `same_task`, `none` — всегда `false`;
- `show_replacement=true` только если модель/устройство явно совпадает.

## 10. Результаты v2

| Тип | Сколько |
|---|---:|
| `substitute` | 23 |
| `none` | 22 |
| `adjacent_task` | 9 |
| `compatibility` | 6 |

`show_cross_sell=true` — 14. Среди них **ноль** заменителей и **ноль** `none`. Это главное улучшение.

На 20 эталонных парах:

- точное совпадение типа — 18/20;
- с допустимыми альтернативами — 20/20;
- после валидатора: TP 4, FP 1, FN 0, TN 15;
- precision 0.8, recall 1.0.

Единственный false positive — Ethernet-кабель и роутер. LLM поставила `compatibility` 0.7, но в карточке роутера явно не сказано про LAN-порты. Если требовать явное упоминание порта/разъёма, это станет `none`.

## 11. Что изменилось

- LLM больше не ставит заменители в кросс-сейл.
- Замена и допродажа разделены.
- Движок всё равно проверяет тип, confidence и evidence.
- `show` из LLM не является решением, только сигналом.

## 12. Что дальше

- ручная проверка 60 пар в `relations-review-v2.csv`;
- precision по каждому типу;
- prompt v3: для `compatibility` требовать явную цитату про порт, цоколь или модель;
- сравнить с правилами без LLM.
