import { api } from '@/shared/api';
import type { Account, AIParseResult, Category, Transaction, TransactionListResponse, TransactionType } from '@/lib/types';

export type TransactionPayload = {
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string[];
  occurredAt: string;
  source: string;
};

export type TransactionFilters = {
  page: number;
  pageSize: number;
  start?: string;
  end?: string;
  type?: '' | TransactionType;
  categoryId?: number;
  accountId?: number;
  q?: string;
};

export function listAccounts() {
  return api.accounts.getAccounts() as Promise<Account[]>;
}

export function listCategories() {
  return api.categories.getCategories({}) as Promise<Category[]>;
}

export function listTransactions(filters: TransactionFilters = { page: 1, pageSize: 20 }) {
  return api.transactions.getTransactions({
    page: filters.page || 1,
    pageSize: filters.pageSize || 20,
    start: filters.start,
    end: filters.end,
    type: filters.type || undefined,
    categoryId: filters.categoryId,
    accountId: filters.accountId,
    q: filters.q?.trim() || undefined,
  }) as Promise<TransactionListResponse>;
}

export function createTransaction(payload: TransactionPayload) {
  return api.transactions.postTransactions(payload) as Promise<Transaction>;
}

export function updateTransaction(id: number, payload: TransactionPayload) {
  return api.transactions.putTransactionsById({ id }, payload) as Promise<Transaction>;
}

export function deleteTransaction(id: number) {
  return api.transactions.deleteTransactionsById({ id });
}

export function parseAIText(text: string) {
  return api.ai.postAiParse({ text }) as Promise<{ result: AIParseResult }>;
}
