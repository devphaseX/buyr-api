CREATE TABLE IF NOT EXISTS option_types (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    created_by_id VARCHAR(50),
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW (),
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW (),
        CONSTRAINT fk_option_types_created_by FOREIGN KEY (created_by_id) REFERENCES admin_users (id)
);

CREATE TABLE IF NOT EXISTS option_values (
    id VARCHAR(50) PRIMARY KEY,
    value VARCHAR(50) NOT NULL UNIQUE,
    display_value VARCHAR(255) NOT NULL,
    created_by_id VARCHAR(50),
    option_type_id VARCHAR(50) NOT NULL,
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW (),
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW (),
        CONSTRAINT fk_option_values_option_type FOREIGN KEY (option_type_id) REFERENCES option_types (id) ON DELETE CASCADE,
        CONSTRAINT fk_option_values_created_by FOREIGN KEY (created_by_id) REFERENCES admin_users (id)
);
