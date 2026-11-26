INSERT INTO relation_types
    (code, title, description, examples, behavior, direction, default_confidence, status, builtin)
VALUES
    ('alternative', 'Альтернатива',
     'Похожий продукт для той же задачи, но не полный аналог. Показывается как альтернатива, а не как замена.',
     'Adobe CC → CorelDRAW для графики; один антивирус → другой антивирус.',
     'alternative', 'forward', 0.8, 'active', true)
ON CONFLICT (code) DO NOTHING;
