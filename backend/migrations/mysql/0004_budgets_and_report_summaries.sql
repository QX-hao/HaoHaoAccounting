CREATE TABLE IF NOT EXISTS budgets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  month VARCHAR(7) NOT NULL,
  category_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY idx_budgets_user_month_category (user_id, month, category_id),
  KEY idx_budgets_user_id (user_id),
  KEY idx_budgets_month (month),
  KEY idx_budgets_category_id (category_id),
  CONSTRAINT fk_budgets_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS daily_summaries (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  day VARCHAR(10) NOT NULL,
  income_cents BIGINT NOT NULL DEFAULT 0,
  expense_cents BIGINT NOT NULL DEFAULT 0,
  tx_count BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY idx_daily_summaries_user_day (user_id, day),
  KEY idx_daily_summaries_user_id (user_id),
  KEY idx_daily_summaries_day (day),
  CONSTRAINT fk_daily_summaries_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS monthly_summaries (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  month VARCHAR(7) NOT NULL,
  income_cents BIGINT NOT NULL DEFAULT 0,
  expense_cents BIGINT NOT NULL DEFAULT 0,
  tx_count BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY idx_monthly_summaries_user_month (user_id, month),
  KEY idx_monthly_summaries_user_id (user_id),
  KEY idx_monthly_summaries_month (month),
  CONSTRAINT fk_monthly_summaries_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
