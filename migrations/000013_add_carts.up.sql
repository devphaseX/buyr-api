CREATE TABLE IF NOT EXISTS carts (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    is_active boolean,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE carts ADD CONSTRAINT carts_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
