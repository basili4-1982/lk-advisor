-- An order instance is (order_id, order_week). The same order_id can appear
-- in different weeks in the source feed, so order_id alone is not the atomic
-- purchase event. The old key collapsed those rows and hid 46 multi-week
-- orders, including 7 that crossed the train/test cut.
ALTER TABLE lk_orders DROP CONSTRAINT IF EXISTS lk_orders_pkey;
ALTER TABLE lk_orders
    ADD CONSTRAINT lk_orders_pkey PRIMARY KEY (order_id, order_week, item_id);

CREATE INDEX IF NOT EXISTS lk_orders_order_week_idx
    ON lk_orders (order_id, order_week);
