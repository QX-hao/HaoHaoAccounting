import { request } from '@/lib/api';
import type { Account, AIParseResult, Category, Transaction, TransactionListResponse, TransactionType } from '@/lib/types';

export type CreateTransactionPayload = {
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string[];
  occurredAt: string;
  source: string;
};

export function listAccounts() {
  return request<Account[]>('/accounts');
}

export function listCategories() {
  return request<Category[]>('/categories');
}

export function listTransactions() {
  return request<TransactionListResponse>('/transactions?page=1&pageSize=50');
}

export function createTransaction(payload: CreateTransactionPayload) {
  return request<Transaction>('/transactions', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function parseAIText(text: string) {
  return request<{ result: AIParseResult }>('/ai/parse', {
    method: 'POST',
    body: JSON.stringify({ text }),
  });
}
