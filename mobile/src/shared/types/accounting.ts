export type Tab = 'home' | 'add' | 'transactions' | 'manage' | 'io' | 'reports' | 'profile';
export type TransactionType = 'income' | 'expense';

export type Account = {
  id: number;
  userId: number;
  name: string;
  balance: number;
  type: string;
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

export type Summary = {
  income: number;
  expense: number;
  balance: number;
  byCategory: { category: string; amount: number }[];
  byAccount: { account: string; amount: number }[];
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
