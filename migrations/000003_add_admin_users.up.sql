CREATE TABLE IF NOT EXISTS admin_users (
    id varchar(50) NOT NULL PRIMARY KEY,
    first_name varchar(255),
    last_name varchar(255),
    user_id varchar(50),
    level varchar(50),
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE admin_users ADD CONSTRAINT admin_users_user_id_unique UNIQUE (user_id);

ALTER TABLE admin_users ADD CONSTRAINT admin_users_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
