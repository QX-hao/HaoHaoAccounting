// Generated from backend/api/openapi.yaml. Do not edit by hand.

export type TransactionType = "income" | "expense";

export type CurrentUser = {
  id: number;
  name: string;
  username: string;
  phone: string;
  email: string;
  wechatId: string;
};

export type LoginRequest = {
  username: string;
  password: string;
};

export type LoginResponse = {
  token: string;
  user: CurrentUser;
};

export type Account = {
  id: number;
  userId: number;
  name: string;
  type: string;
  balance: number;
  createdAt: string;
  updatedAt: string;
};

export type AccountRequest = {
  name: string;
  type: string;
  balance?: number;
};

export type Budget = {
  id: number;
  userId: number;
  month: string;
  categoryId: number;
  amount: number;
  createdAt: string;
  updatedAt: string;
};

export type BudgetRequest = {
  month: string;
  categoryId: number;
  amount: number;
};

export type Category = {
  id: number;
  userId?: number;
  name: string;
  type: TransactionType;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
};

export type CategoryRequest = {
  name: string;
  type: TransactionType;
};

export type Transaction = {
  id: number;
  userId: number;
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string;
  source: string;
  occurredAt: string;
  category: Category;
  account: Account;
  createdAt: string;
  updatedAt: string;
};

export type TransactionRequest = {
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags?: string[];
  occurredAt?: string;
  source?: string;
};

export type TransactionListResponse = {
  items: Transaction[];
  pagination: Pagination;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
};

export type AIParseRequest = {
  text: string;
};

export type AIParseResult = {
  type: TransactionType;
  amount: number;
  category: string;
  account: string;
  note: string;
  occurredAt: string;
  confidence: number;
};

export type AIParseResponse = {
  requiresConfirmation: boolean;
  cached: boolean;
  result: AIParseResult;
};

export type CategoryStat = {
  categoryId?: number;
  category: string;
  amount: number;
};

export type AccountStat = {
  accountId?: number;
  account: string;
  amount: number;
};

export type MonthTrend = {
  month: string;
  income: number;
  expense: number;
};

export type TrendPoint = {
  period: string;
  income: number;
  expense: number;
};

export type CategoryTrendPoint = {
  period: string;
  categoryId: number;
  category: string;
  amount: number;
};

export type AccountBalancePoint = {
  period: string;
  accountId: number;
  account: string;
  net: number;
  balance: number;
};

export type BudgetExecution = {
  budgetId: number;
  month: string;
  categoryId: number;
  category: string;
  budget: number;
  expense: number;
  remaining: number;
  usageRate: number;
};

export type SummaryTableRow = {
  period: string;
  income: number;
  expense: number;
  balance: number;
  txCount: number;
};

export type PeriodTotals = {
  income: number;
  expense: number;
};

export type PeriodCompare = {
  current: PeriodTotals;
  previous: PeriodTotals;
};

export type Summary = {
  start: string;
  end: string;
  income: number;
  expense: number;
  balance: number;
  byCategory: CategoryStat[];
  byAccount: AccountStat[];
  monthlyTrend: MonthTrend[];
  trendGranularity: string;
  trend: TrendPoint[];
  categoryTrend: CategoryTrendPoint[];
  accountBalanceTrend: AccountBalancePoint[];
  budgetExecution: BudgetExecution[];
  dailySummaries: SummaryTableRow[];
  monthlySummaries: SummaryTableRow[];
  periodCompare: PeriodCompare;
};

export type ImportFileRequest = {
  file: unknown;
  skipDuplicates?: boolean;
};

export type ImportTextRequest = {
  filename?: string;
  content: string;
  skipDuplicates?: boolean;
};

export type ImportPreviewRow = {
  line: number;
  occurredAt: string;
  type: TransactionType;
  amount: number;
  category: string;
  account: string;
  note: string;
  tags: string;
  valid: boolean;
  duplicate: boolean;
  error?: string;
  duplicateReason?: string;
};

export type ImportPreview = {
  filename: string;
  size: number;
  totalRows: number;
  validRows: number;
  failedRows: number;
  duplicateRows: number;
  maxRows: number;
  maxFileBytes: number;
  truncated: boolean;
  rows: ImportPreviewRow[];
};

export type ImportResult = {
  total: number;
  success: number;
  failed: number;
  skipped: number;
  errors: string[];
};

export type ImportJob = {
  id: number;
  filename: string;
  status: "queued" | "running" | "completed" | "failed";
  total: number;
  success: number;
  failed: number;
  skipped: number;
  errors: string[];
  createdAt: string;
  updatedAt: string;
};

export type ErrorResponse = {
  error: string;
  code: "bad_request" | "invalid_request" | "unauthorized" | "forbidden" | "not_found" | "method_not_allowed" | "rate_limited" | "payload_too_large" | "unsupported_media_type" | "not_acceptable" | "request_timeout" | "client_closed_request" | "internal_error";
  status: number;
  requestId: string;
};

export type OkResponse = {
  ok: boolean;
};

