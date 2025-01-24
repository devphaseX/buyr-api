CREATE TABLE IF NOT EXISTS carts (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone,
        is_active boolean
);

ALTER TABLE carts ADD CONSTRAINT carts_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
