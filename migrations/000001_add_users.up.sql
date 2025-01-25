CREATE TABLE IF NOT EXISTS users (
    id varchar(50) PRIMARY KEY,
    email varchar(255),
    password_hash bytea,
    avatar_url text,
    role varchar(50),
    email_verified_at timestamp,
    is_active boolean default true,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE users ADD CONSTRAINT unique_email UNIQUE (email);
