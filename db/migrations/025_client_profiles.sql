-- Client profiles and hypothesis emulation runs.
-- A profile is a described client (audience, industry, free-text needs) used to
-- replay sessions against a catalog relation without real traffic.

CREATE TABLE IF NOT EXISTS client_profiles (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    audience    text NOT NULL DEFAULT 'b2b',
    industry    text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS client_profiles_audience_idx ON client_profiles (audience, industry);

CREATE TABLE IF NOT EXISTS emulation_runs (
    id            text PRIMARY KEY,
    experiment_id text NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    profile_id    text NOT NULL REFERENCES client_profiles(id) ON DELETE CASCADE,
    impressions   integer NOT NULL,
    seed          bigint NOT NULL DEFAULT 0,
    fit           numeric(5,4) NOT NULL DEFAULT 0,
    summary       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS emulation_runs_experiment_idx
    ON emulation_runs (experiment_id, created_at DESC);

INSERT INTO client_profiles (id, name, audience, industry, description) VALUES
('prof-clinic', 'Стоматологическая клиника', 'b2b', 'Медицинские организации',
 'Клиника на 15 сотрудников. Нужны корпоративная почта, антивирус, защита персональных данных, офисные программы и резервное копирование. Бюджет ограничен, закупка через тендер.'),
('prof-school', 'Средняя школа', 'b2b', 'Образование и наука',
 'Школа: 60 сотрудников и 900 учеников. Нужны офисные программы, электронная почта, системы контроля доступа и защита данных. Закупка по 44-ФЗ.'),
('prof-retail', 'Розничная сеть', 'b2b', 'Розничные сети и оптовая торговля',
 'Сеть из 40 магазинов. Нужны кассовое ПО, учёт склада, аналитика продаж и защита данных. Важна интеграция с 1С.'),
('prof-home', 'Частный покупатель', 'b2c', '',
 'Покупатель для дома. Ищет офисные программы, антивирус и простой графический редактор для дизайна. Смотрит на цену и простоту установки.')
ON CONFLICT (id) DO NOTHING;
