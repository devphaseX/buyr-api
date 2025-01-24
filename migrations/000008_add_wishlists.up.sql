CREATE TABLE IF NOT EXISTS wishlists (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    product_id varchar(50),
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE wishlists ADD CONSTRAINT wishlists_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id),
ADD CONSTRAINT wishlists_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
