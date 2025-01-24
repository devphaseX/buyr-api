CREATE TABLE IF NOT EXISTS audits (
    id varchar(500) NOT NULL PRIMARY KEY,
    admin_id varchar(500),
    action varchar(500),
    details text,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);
