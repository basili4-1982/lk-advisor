# Таксономия задач и потребностей

Профиль строится не по свободным тегам, а по иерархии: домен → категория → задача. Словарь расширяемый, но канонический.

## 1. Уровни

```text
домен
  категория
    задача
      модальность / атрибут
```

Пример:

```text
software
  graphics
    raster
    vector
    photo
    video
  3d
```

## 2. Домены

| ID | Название | Примеры |
|---|---|---|
| `software` | ПО | лицензии, подписки |
| `hardware` | Железо | ПК, серверы, аксессуары |
| `cloud` | Облака | IaaS, SaaS, резервное копирование |
| `services` | Услуги | поддержка, обучение, внедрение |

## 3. Категории

| ID | Название | Примеры |
|---|---|---|
| `games` | Игры | игровые лицензии, подписки |
| `graphics` | Графика | растровые и векторные редакторы |
| `security` | Безопасность | антивирус, защита endpoint |
| `backup` | Резервное копирование | бэкап, восстановление |
| `office` | Офис | документы, таблицы, презентации |
| `devtools` | Разработка | IDE, CI/CD, тестирование |
| `analytics` | Аналитика | BI, отчёты, данные |
| `communication` | Коммуникации | почта, мессенджеры, ВКС |
| `virtualization` | Виртуализация | гипервизоры, контейнеры |
| `training` | Обучение | курсы, сертификации |

## 4. Задачи

| ID | Категория | Что решает | Алиасы |
|---|---|---|---|
| `gaming` | games | играть | игры, gaming, игровые |
| `game_dev` | games | делать игры | gamedev, разработка игр |
| `raster` | graphics | обрабатывать растровые изображения | растр, фотошоп, bitmap |
| `vector` | graphics | логотипы, иконки, вектор | вектор, illustrator |
| `photo` | graphics | обрабатывать фото | фотомонтаж, retouch |
| `video` | graphics | монтировать видео | видеомонтаж, editing |
| `3d` | graphics | 3D-модели | 3д, modeling |
| `endpoint_security` | security | защищать устройства | антивирус, EDR |
| `data_protection` | backup | защищать данные | бэкап, backup |
| `recovery` | backup | восстанавливать после сбоя | disaster recovery |
| `documents` | office | документы | Word, тексты |
| `spreadsheets` | office | таблицы | Excel |
| `presentations` | office | презентации | PowerPoint |
| `coding` | devtools | писать код | IDE, разработка |
| `testing` | devtools | тестировать | QA, автотесты |
| `reporting` | analytics | отчёты | BI, дашборды |
| `data_analysis` | analytics | анализировать данные | аналитика |
| `email` | communication | почта | mail |
| `messaging` | communication | чаты | мессенджер |
| `video_calls` | communication | видеозвонки | ВКС |
| `virtualization` | virtualization | виртуализация | гипервизор |
| `containers` | virtualization | контейнеры | Docker, Kubernetes |
| `training` | training | обучение | курсы |

## 5. Связи задач

| Тип | Пример | Смысл |
|---|---|---|
| `same_task` | raster → raster | замена или апгрейд |
| `adjacent_task` | raster → vector | дополнение |
| `workflow_next_step` | photo → video | следующий шаг |
| `substitute` | raster A → raster B | конкурент |
| `upgrade` | тариф 10 мест → 50 мест | рост |
| `accessory` | редактор → планшет | аксессуар |
| `compatibility` | плагин → редактор | совместимость |

## 6. Канонизация

- Алиасы маппятся на канонический ID.
- Новый тег от LLM попадает в карантин.
- В рекомендации идёт только канон.
- Если задача не найдена — работаем на уровне категории.
- Если и категории нет — `unknown`, рекомендация не строится.

## 7. Пример: игры

Клиент купил три игры и геймпад.

```json
{
  "affinities": [
    {"type": "category", "id": "games", "score": 0.9},
    {"type": "task", "id": "gaming", "score": 0.8},
    {"type": "category", "id": "hardware", "score": 0.4}
  ],
  "needs": [
    {"tag": "entertainment", "score": 0.8, "evidence": "3 покупки игр"}
  ]
}
```

Кандидаты: игровые подписки, аксессуары, апгрейд видеокарты. Не backup и не офис.
