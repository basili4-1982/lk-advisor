UPDATE external_sources
SET status = 'connected',
    limitation = 'TLS-сертификат Минцифры и User-Agent; ODS-выгрузка',
    updated_at = now()
WHERE id = 'fstec';
