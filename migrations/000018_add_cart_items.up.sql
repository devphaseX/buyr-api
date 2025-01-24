CREATE TABLE IF NOT EXISTS cart_items (
    id varchar(50) NOT NULL PRIMARY KEY,
    cart_id varchar(50),
    product_id varchar(50),
    quantity integer,
    added_at timestamp
    with
        time zone,
        quantity integer,
        created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE cart_items ADD CONSTRAINT cart_items_cart_id_fk FOREIGN KEY (cart_id) REFERENCES carts (id),
ADD CONSTRAINT cart_items_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
