CREATE TABLE IF NOT EXISTS product_features (
    id varchar(50) NOT NULL PRIMARY KEY,
    title varchar(255),
    view varchar(50),
    feature_entries jsonb,
    product_id varchar(50),
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE product_features ADD CONSTRAINT product_features_product_id_fk FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;
