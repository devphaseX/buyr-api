CREATE TABLE IF NOT EXISTS orders (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    total_amount decimal(10, 2),
    promo_code varchar(20),
    discount decimal,
    status varchar(50)
    -- '"pending","processing", "shipped", "delivered", "cancelled"'
,
    paid boolean,
    payment_method varchar(50),
    created_at timestamp
    with
        time zone,
        updated_at timestamp
);

ALTER TABLE orders ADD CONSTRAINT orders_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL;
