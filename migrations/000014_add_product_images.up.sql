CREATE TABLE IF NOT EXISTS product_images (
    id varchar(50) NOT NULL PRIMARY KEY,
    product_id varchar(50),
    url text,
    is_primary boolean,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE product_images ADD CONSTRAINT product_images_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
