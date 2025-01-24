CREATE TABLE IF NOT EXISTS addresses (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    address_type varchar(50),
    street_address text,
    city varchar(50),
    state varchar(50),
    postal_code varchar(20),
    country varchar(50),
    is_default boolean,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

-- COMMENT ON COLUMN addresses.address_type IS '"home", "work", "other"';
ALTER TABLE addresses ADD CONSTRAINT addresses_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
