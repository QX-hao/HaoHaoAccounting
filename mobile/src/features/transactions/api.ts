import { api } from '../../shared/api';
import type { AIParseResult, Transaction, TransactionListResponse, TransactionType } from '../../shared/types/accounting';

export type TransactionPayload = {
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string[];
  source: string;
  occurredAt: string;
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

export function listTransactions(filters: TransactionFilters) {
  return api.transactions.getTransactions({
    page: filters.page,
    pageSize: filters.pageSize,
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
