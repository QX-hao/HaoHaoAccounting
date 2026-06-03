CREATE INDEX IF NOT EXISTS idx_transactions_user_occurred ON transactions (user_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_type_occurred ON transactions (user_id, type, occurred_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_category_occurred ON transactions (user_id, category_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_account_occurred ON transactions (user_id, account_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_duplicate_lookup ON transactions (user_id, occurred_at, type, amount_cents, category_id, account_id);
