ALTER TABLE order_items
ADD COLUMN IF NOT EXISTS cart_item_id VARCHAR(50)
