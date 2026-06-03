CREATE TABLE IF NOT EXISTS budgets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  month VARCHAR(7) NOT NULL,
  category_id BIGINT NOT NULL DEFAULT 0,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_user_month_category ON budgets (user_id, month, category_id);
CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets (user_id);
CREATE INDEX IF NOT EXISTS idx_budgets_month ON budgets (month);
CREATE INDEX IF NOT EXISTS idx_budgets_category_id ON budgets (category_id);

CREATE TABLE IF NOT EXISTS daily_summaries (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  day VARCHAR(10) NOT NULL,
  income_cents BIGINT NOT NULL DEFAULT 0,
  expense_cents BIGINT NOT NULL DEFAULT 0,
  tx_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_summaries_user_day ON daily_summaries (user_id, day);
CREATE INDEX IF NOT EXISTS idx_daily_summaries_user_id ON daily_summaries (user_id);
CREATE INDEX IF NOT EXISTS idx_daily_summaries_day ON daily_summaries (day);

CREATE TABLE IF NOT EXISTS monthly_summaries (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  month VARCHAR(7) NOT NULL,
  income_cents BIGINT NOT NULL DEFAULT 0,
  expense_cents BIGINT NOT NULL DEFAULT 0,
  tx_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monthly_summaries_user_month ON monthly_summaries (user_id, month);
CREATE INDEX IF NOT EXISTS idx_monthly_summaries_user_id ON monthly_summaries (user_id);
CREATE INDEX IF NOT EXISTS idx_monthly_summaries_month ON monthly_summaries (month);
