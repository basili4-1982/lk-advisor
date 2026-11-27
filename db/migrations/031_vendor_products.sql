-- External product knowledge: vendor products at product/edition level and
-- their mapping to catalog SKUs. This is the entity-resolution layer that the
-- vendor-page probe showed to be the real bottleneck.

CREATE TABLE IF NOT EXISTS vendor_products (
    id              bigserial PRIMARY KEY,
    vendor          text NOT NULL,
    product         text NOT NULL,
    family          text NOT NULL DEFAULT '',
    edition         text NOT NULL DEFAULT '',
    platforms       jsonb NOT NULL DEFAULT '[]'::jsonb,
    registry_number text NOT NULL DEFAULT '',
    source_url      text NOT NULL DEFAULT '',
    aliases         jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (vendor, product, edition)
);

CREATE TABLE IF NOT EXISTS vendor_product_items (
    vendor_product_id bigint NOT NULL REFERENCES vendor_products(id) ON DELETE CASCADE,
    item_id           text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    method            text NOT NULL DEFAULT '',
    confidence        numeric(4,3) NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (vendor_product_id, item_id)
);

CREATE INDEX IF NOT EXISTS vendor_product_items_item_idx
    ON vendor_product_items (item_id);

INSERT INTO vendor_products (vendor, product, family, edition, platforms, source_url, aliases) VALUES
('Google', 'Google Workspace', 'Google Workspace', 'Starter', '["Web","Mobile"]',
 'https://workspace.google.com/pricing.html', '["Google Workspace Starter"]'),
('Google', 'Google Workspace', 'Google Workspace', 'Standard', '["Web","Mobile"]',
 'https://workspace.google.com/pricing.html', '["Google Workspace Business Standard","Google Workspace Standard"]'),
('Google', 'Google Workspace', 'Google Workspace', 'Plus', '["Web","Mobile"]',
 'https://workspace.google.com/pricing.html', '["Google Workspace Business Plus","Google Workspace Plus"]'),
('Google', 'Google Workspace', 'Google Workspace', 'Enterprise', '["Web","Mobile"]',
 'https://workspace.google.com/pricing.html', '["Google Workspace Enterprise"]'),
('КриптоПро', 'КриптоПро CSP', 'КриптоПро CSP', '', '["Windows","macOS","Linux","FreeBSD","Solaris","AIX","iOS","Android"]',
 'https://www.cryptopro.ru/products/csp', '["КриптоПро CSP"]'),
('КриптоПро', 'КриптоПро JCP', 'КриптоПро JCP', '', '["Java"]',
 'https://www.cryptopro.ru/products/jcp', '["КриптоПро JCP"]'),
('Dr.Web', 'Dr.Web Desktop Security Suite', 'Dr.Web Security Suite', '', '["Windows","macOS","Linux"]',
 'https://drweb.ru/', '["Dr.Web Desktop Security Suite"]'),
('Dr.Web', 'Dr.Web Server Security Suite', 'Dr.Web Security Suite', '', '["Windows","Linux"]',
 'https://drweb.ru/', '["Dr.Web Server Security Suite"]'),
('Dr.Web', 'Dr.Web Enterprise Security Suite', 'Dr.Web Security Suite', '', '["Windows","Linux"]',
 'https://drweb.ru/', '["Dr.Web Enterprise Security Suite"]'),
('1С-Битрикс', '1С-Битрикс: Управление сайтом', '1С-Битрикс', 'Старт', '["Web"]',
 'https://www.1c-bitrix.ru/products/cms/', '["1С-Битрикс: Управление сайтом. Лицензия Старт"]'),
('1С-Битрикс', '1С-Битрикс: Управление сайтом', '1С-Битрикс', 'Стандарт', '["Web"]',
 'https://www.1c-bitrix.ru/products/cms/', '["1С-Битрикс: Управление сайтом. Лицензия Стандарт"]'),
('1С-Битрикс', '1С-Битрикс: Управление сайтом', '1С-Битрикс', 'Малый бизнес', '["Web"]',
 'https://www.1c-bitrix.ru/products/cms/', '["1С-Битрикс: Управление сайтом. Лицензия Малый бизнес"]'),
('1С-Битрикс', '1С-Битрикс: Управление сайтом', '1С-Битрикс', 'Бизнес', '["Web"]',
 'https://www.1c-bitrix.ru/products/cms/', '["1С-Битрикс: Управление сайтом. Лицензия Бизнес"]'),
('Astra Linux', 'Astra Linux Special Edition', 'Astra Linux', '', '["Linux"]',
 'https://astralinux.ru/products/astra-linux-special-edition/', '["Astra Linux Special Edition"]'),
('VK', 'VK WorkSpace', 'VK WorkSpace', 'Базовый', '["Windows","Linux","Web","Mobile"]',
 'https://workspace.vk.ru/', '["VK WorkMail","VK WorkSpace"]'),
('VK', 'VK WorkSpace', 'VK WorkSpace', 'Расширенный', '["Windows","Linux","Web","Mobile"]',
 'https://workspace.vk.ru/', '["VK WorkSpace Расширенный"]')
ON CONFLICT (vendor, product, edition) DO NOTHING;
