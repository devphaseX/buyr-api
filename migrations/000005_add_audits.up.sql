CREATE TABLE audit_events (
    id VARCHAR(50) PRIMARY KEY,
    event_type TEXT NOT NULL,
    account_id VARCHAR(50),
    performed_by VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL, -- Reason for the event (e.g., "violation of terms")
    details BYTEA, -- Additional details about the event (stored as binary data)
    timestamp TIMESTAMP
    WITH
        TIME ZONE NOT NULL, -- Timestamp of the event
        admin_level_access INT NOT NULL, -- Admin level of the user who performed the action
        ip_address TEXT, -- IP address of the user who performed the action
        user_agent TEXT, -- User agent of the client (e.g., browser or device)
        created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- When the record was created
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP -- When the record was last updated
);
