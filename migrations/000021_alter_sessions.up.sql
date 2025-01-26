ALTER TABLE sessions
ADD COLUMN updated_at TIMESTAMP
WITH
    TIME ZONE DEFAULT NOW ();

CREATE TRIGGER update_sessions_updated_at BEFORE
UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column ();
