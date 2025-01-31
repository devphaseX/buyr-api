-- Drop the unique constraint from the transaction_id field
ALTER TABLE payments
DROP CONSTRAINT IF EXISTS payments_transaction_id_unique;
