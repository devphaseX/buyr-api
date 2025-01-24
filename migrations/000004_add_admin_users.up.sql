CREATE TABLE IF NOT EXISTS admin_users (
    id varchar(50) NOT NULL PRIMARY KEY,
    first_name varchar(255),
    last_name varchar(255),
    user_id varchar(50),
    auth_secret text,
    two_factor_auth_enabled boolean,
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE admin_users ADD CONSTRAINT admin_users_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id);
