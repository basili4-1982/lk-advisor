-- Editable relation templates. Descriptions and examples are injected into the
-- relation-extraction prompt; behavior tells the recommendation engine how to use a link.
CREATE TABLE IF NOT EXISTS relation_types (
    code               text PRIMARY KEY,
    title              text NOT NULL,
    description        text NOT NULL DEFAULT '',
    examples           text NOT NULL DEFAULT '',
    behavior           text NOT NULL DEFAULT 'info',
    direction          text NOT NULL DEFAULT 'forward',
    default_confidence numeric(4,3) NOT NULL DEFAULT 0.8,
    status             text NOT NULL DEFAULT 'active',
    builtin            boolean NOT NULL DEFAULT false,
    updated_at         timestamptz NOT NULL DEFAULT now()
);

INSERT INTO relation_types
    (code, title, description, examples, behavior, direction, default_confidence, status, builtin)
VALUES
    ('upgrade', 'Апгрейд', 'Старшая версия или старший тариф того же продукта. Покупатель получает больше возможностей, а не другой продукт.', 'Visio Standard → Visio Professional; КОМПАС-График → КОМПАС-3D; Microsoft 365 бизнес стандарт → бизнес премиум.', 'upsell', 'forward', 0.9, 'active', true),
    ('bundle', 'Комплект', 'Два продукта выгоднее или удобнее продавать вместе; один усиливает другой, но не заменяет его.', 'Office + PDF Toolkit; КриптоАРМ + Office; антивирус + резервное копирование.', 'bundle', 'forward', 0.9, 'active', true),
    ('adjacent_task', 'Дополнение', 'Продукт закрывает соседнюю задачу в том же рабочем процессе. Это не замена и не старшая версия.', 'Visio → Project; BI Studio → Report Builder; Office → курс по Office.', 'cross_sell', 'forward', 0.8, 'active', true),
    ('accessory', 'Аксессуар', 'Вспомогательный товар, который нужен для работы основного: расходник, модуль, лицензия на рабочее место.', 'ПО → дополнительная лицензия; оборудование → кабель; платформа → модуль.', 'cross_sell', 'forward', 0.8, 'active', true),
    ('compatibility', 'Совместимость', 'Продукты явно совместимы по API, формату, разъёму или модели. Связь симметричная: если A совместим с B, то и B с A.', '1С → модуль обмена; САПР → плагин; кабель → роутер с тем же разъёмом.', 'cross_sell', 'symmetric', 0.8, 'active', true),
    ('workflow_next_step', 'Следующий шаг', 'После покупки клиенту логично перейти к следующему этапу: внедрение, обучение, миграция, резервное копирование.', 'ПО → курс; внедрение → аудит; миграция → сопровождение.', 'cross_sell', 'forward', 0.8, 'active', true),
    ('substitute', 'Замена', 'Конкурент или другой продукт для той же задачи. Прямо не допродаётся; показывается только как бюджетная альтернатива при более низкой цене или акции.', 'GStarCAD ↔ КОМПАС-3D; Kaiten ↔ Яндекс Трекер; один антивирус ↔ другой антивирус.', 'substitute', 'forward', 0.9, 'active', true),
    ('same_task', 'Та же задача', 'Другой продукт для той же задачи, но не обязательно прямой конкурент. В допродажу не идёт.', 'два растровых редактора; две CRM; два средства резервного копирования.', 'substitute', 'forward', 0.9, 'active', true),
    ('analog', 'Аналог снятого', 'Замена продукта, который снят с продажи или больше не поддерживается.', 'снятый пакет → актуальный пакет; устаревшая версия → поддерживаемая версия.', 'analog', 'forward', 0.8, 'active', true),
    ('excludes', 'Несовместимость', 'Продукты нельзя использовать или предлагать вместе. Это ограничение, а не рекомендация.', 'два антивируса с резидентными модулями; два продукта с конфликтом драйверов.', 'info', 'symmetric', 0.9, 'active', true)
ON CONFLICT (code) DO NOTHING;
