-- FSTEC certified security tools registry (ODS export).
CREATE TABLE IF NOT EXISTS fstec_certificates (
    id                bigserial PRIMARY KEY,
    certificate_number text NOT NULL,
    registered_at     date,
    valid_until       date,
    product           text NOT NULL,
    requirements      text NOT NULL DEFAULT '',
    scheme            text NOT NULL DEFAULT '',
    lab               text NOT NULL DEFAULT '',
    body              text NOT NULL DEFAULT '',
    applicant         text NOT NULL DEFAULT '',
    address           text NOT NULL DEFAULT '',
    support_until     date,
    security_support  text NOT NULL DEFAULT '',
    source            text NOT NULL DEFAULT 'fstec_ods',
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (certificate_number, product, applicant)
);

CREATE TABLE IF NOT EXISTS fstec_certificate_items (
    certificate_id bigint NOT NULL REFERENCES fstec_certificates(id) ON DELETE CASCADE,
    item_id        text NOT NULL REFERENCES lk_items(item_id) ON DELETE CASCADE,
    method         text NOT NULL DEFAULT '',
    confidence     numeric(4,3) NOT NULL DEFAULT 0,
    PRIMARY KEY (certificate_id, item_id)
);

CREATE INDEX IF NOT EXISTS fstec_certificate_items_item_idx
    ON fstec_certificate_items (item_id);
