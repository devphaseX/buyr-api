CREATE TABLE IF NOT EXISTS products (
    id varchar(500) NOT NULL PRIMARY KEY,
    name varchar(500),
    description text,
    stock_quantity integer,
    total_items_sold_count integer,
    vendor_id varchar(500),
    discount decimal(10, 2),
    price decimal(10, 2),
    category_id varchar(500),
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE products ADD CONSTRAINT products_category_id_fk FOREIGN KEY (category_id) REFERENCES category (id) ON DELETE SET NULL,
ADD CONSTRAINT products_vendor_id_fk FOREIGN KEY (vendor_id) REFERENCES vendor_users (id) ON DELETE CASCADE;
