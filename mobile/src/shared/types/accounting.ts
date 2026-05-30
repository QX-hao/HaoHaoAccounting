export type Tab = 'home' | 'add' | 'reports' | 'profile';
export type TransactionType = 'income' | 'expense';

export type Account = {
  id: number;
  name: string;
  balance: number;
  type: string;
};

export type Category = {
  id: number;
  name: string;
  type: TransactionType;
  isSystem: boolean;
};

export type Transaction = {
  id: number;
  type: TransactionType;
  amount: number;
  note: string;
  occurredAt: string;
  category: { name: string };
  account: { name: string };
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
};
