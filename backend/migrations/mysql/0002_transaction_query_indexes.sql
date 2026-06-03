ALTER TABLE transactions ADD KEY idx_transactions_user_occurred (user_id, occurred_at);
ALTER TABLE transactions ADD KEY idx_transactions_user_type_occurred (user_id, type, occurred_at);
ALTER TABLE transactions ADD KEY idx_transactions_user_category_occurred (user_id, category_id, occurred_at);
ALTER TABLE transactions ADD KEY idx_transactions_user_account_occurred (user_id, account_id, occurred_at);
ALTER TABLE transactions ADD KEY idx_transactions_user_duplicate_lookup (user_id, occurred_at, type, amount_cents, category_id, account_id);
