-- Demo evidence: show that a relation can be backed by a vendor page, a
-- product page and a catalog attribute, not only by co-purchase counts.
UPDATE relations
SET evidence = evidence || '[{"kind":"vendor_doc","summary":"Google Workspace: переход между тарифами","details":{"url":"https://support.google.com/a/answer/60215"}},{"kind":"product_page","summary":"Google Workspace Starter: состав и тарифы","details":{"url":"https://workspace.google.com/pricing.html"}},{"kind":"attribute","summary":"Одна линейка Google Workspace: Business Standard и Starter","details":{"product_group":"S-GOOGLE-WORKSPACE-STARTER"}}]'::jsonb
WHERE from_item_id = 'GAPPS-BUS-STD-1USER-1MO-First-Year'
  AND to_item_id = 'GAPPS-STARTER-1USER-1MO-First-Year'
  AND NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements(evidence) e WHERE e->>'kind' = 'vendor_doc'
  );

UPDATE relations
SET evidence = evidence || '[{"kind":"vendor_doc","summary":"Документация вендора «Ревизор»","details":{"url":"https://www.revizor.ru/"}},{"kind":"product_page","summary":"Описание продукта «Ревизор»","details":{"url":"https://www.revizor.ru/products/"}}]'::jsonb
WHERE from_item_id = '17065' AND to_item_id = '17067'
  AND NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements(evidence) e WHERE e->>'kind' = 'vendor_doc'
  );
