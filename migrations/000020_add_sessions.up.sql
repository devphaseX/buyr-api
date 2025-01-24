CREATE TABLE IF NOT EXISTS sessions (
    id varchar(255) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    user_agent varchar(255) NOT NULL,
    ip varchar(45) NOT NULL,
    version integer DEFAULT 1,
    expires_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL,
        last_used TIMESTAMP
    WITH
        TIME ZONE,
        created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW (),
        remember_me bool DEFAULT false,
        max_renewal_duration integer
);

-- Add foreign key constraint with ON DELETE CASCADE
ALTER TABLE sessions ADD CONSTRAINT sessions_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
