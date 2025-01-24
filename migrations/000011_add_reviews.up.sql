CREATE TABLE IF NOT EXISTS reviews (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    product_id varchar(50),
    rating integer,
    comment text,
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE reviews ADD CONSTRAINT reviews_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id),
ADD CONSTRAINT reviews_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
