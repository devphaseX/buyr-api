CREATE TABLE IF NOT EXISTS vendor_users (
    id varchar(50) NOT NULL PRIMARY KEY,
    business_name varchar(255),
    business_address text,
    contact_number varchar(20),
    user_id varchar(50),
    approved_at timestamp,
    suspended_at timestamp,
    created_by_admin_id varchar(50),
    created_at timestamp
    with
        time zone,
        updated_at timestamp
    with
        time zone
);

ALTER TABLE vendor_users ADD CONSTRAINT vendor_users_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id),
ADD CONSTRAINT vendor_users_created_by_admin_id_fk FOREIGN KEY (created_by_admin_id) REFERENCES admin_users (id);
