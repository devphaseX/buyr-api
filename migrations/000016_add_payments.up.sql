CREATE TABLE IF NOT EXISTS payments (
    id varchar(50) NOT NULL PRIMARY KEY,
    order_id varchar(50),
    payment_method varchar(50),
    amount decimal(10, 2),
    status varchar(50),
    transaction_id varchar(50),
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

-- COMMENT ON COLUMN payments.payment_method IS '"credit_card", "stripe", "bank_transfer"';
-- COMMENT ON COLUMN payments.status IS '"pending", "completed", "failed"';
-- COMMENT ON COLUMN payments.transaction_id IS '255';
ALTER TABLE payments ADD CONSTRAINT payments_order_id_fk FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE SET NULL;
