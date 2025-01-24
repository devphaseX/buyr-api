CREATE TABLE IF NOT EXISTS order_items (
    id varchar(50) NOT NULL PRIMARY KEY,
    order_id varchar(50),
    product_id varchar(50),
    quantity integer,
    price decimal
);

ALTER TABLE order_items ADD CONSTRAINT order_items_order_id_fk FOREIGN KEY (order_id) REFERENCES orders (id),
ADD CONSTRAINT order_items_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
