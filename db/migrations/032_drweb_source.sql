-- The product page lists Desktop/Server/Enterprise as components of the suite.
UPDATE vendor_products
SET source_url = 'https://products.drweb.ru/ess_universal/',
    updated_at = now()
WHERE vendor = 'Dr.Web';
