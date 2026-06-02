export type TransactionType = 'income' | 'expense';

export type Account = {
  id: number;
  userId: number;
  name: string;
  type: string;
  balance: number;
};

export type Category = {
  id: number;
  userId?: number;
  name: string;
  type: TransactionType;
  isSystem: boolean;
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
};

export type TransactionListResponse = {
  items: Transaction[];
  pagination?: {
    page: number;
    pageSize: number;
    total: number;
  };
};

export type CategoryStat = { category: string; amount: number };
export type AccountStat = { account: string; amount: number };
export type MonthTrend = { month: string; income: number; expense: number };

export type Summary = {
  income: number;
  expense: number;
  balance: number;
  byCategory: CategoryStat[];
  byAccount: AccountStat[];
  monthlyTrend?: MonthTrend[];
  periodCompare?: {
    current: { income: number; expense: number };
    previous: { income: number; expense: number };
  };
};

export type CurrentUser = {
  id: number;
  name: string;
  username?: string;
  phone?: string;
  email?: string;
  wechatId?: string;
};

export type AIParseResult = {
  type: TransactionType;
  amount: number;
  category: string;
  account: string;
  note: string;
  occurredAt: string;
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
  error?: string;
};

export type ImportPreview = {
  filename: string;
  size: number;
  totalRows: number;
  validRows: number;
  failedRows: number;
  maxRows: number;
  maxFileBytes: number;
  truncated: boolean;
  rows: ImportPreviewRow[];
};

export type ImportResult = {
  total: number;
  success: number;
  failed: number;
  errors: string[];
};
