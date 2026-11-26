-- Client audience is passed by the personal cabinet when it opens.
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS audience text NOT NULL DEFAULT 'b2b';

CREATE INDEX IF NOT EXISTS clients_audience_idx ON clients (audience);
