CREATE TABLE IF NOT EXISTS notifications (
    id varchar(50) NOT NULL PRIMARY KEY,
    user_id varchar(50),
    message text,
    is_read boolean,
    created_at timestamp
    with
        time zone default now (),
        updated_at timestamp
    with
        time zone default now ()
);

ALTER TABLE notifications ADD CONSTRAINT notifications_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
