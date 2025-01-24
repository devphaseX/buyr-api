CREATE TABLE IF NOT EXISTS category (
    id varchar(50) NOT NULL PRIMARY KEY,
    name varchar(255),
    description text,
    created_by_admin_id varchar(50),
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE category ADD CONSTRAINT category_created_by_admin_id_fk FOREIGN KEY (created_by_admin_id) REFERENCES admin_users (id) ON DELETE SET NULL;
