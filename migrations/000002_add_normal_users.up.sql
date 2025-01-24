CREATE TABLE IF NOT EXISTS normal_users (
    id varchar(50) NOT NULL PRIMARY KEY,
    first_name varchar(255),
    last_name varchar(255),
    phone_number varchar(20),
    user_id varchar(50),
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE normal_users ADD CONSTRAINT normal_users_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
