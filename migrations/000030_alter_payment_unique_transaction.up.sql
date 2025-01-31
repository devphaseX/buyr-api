-- Add a unique constraint to the transaction_id field
ALTER TABLE payments ADD CONSTRAINT payments_transaction_id_unique UNIQUE (transaction_id);
