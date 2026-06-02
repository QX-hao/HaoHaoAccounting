import { request } from '@/lib/api';
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
  return request<Account[]>('/accounts');
}

export function listCategories() {
  return request<Category[]>('/categories');
}

export function listTransactions(filters: TransactionFilters = { page: 1, pageSize: 20 }) {
  const params = new URLSearchParams({
    page: String(filters.page || 1),
    pageSize: String(filters.pageSize || 20),
  });
  if (filters.start) params.set('start', filters.start);
  if (filters.end) params.set('end', filters.end);
  if (filters.type) params.set('type', filters.type);
  if (filters.categoryId) params.set('categoryId', String(filters.categoryId));
  if (filters.accountId) params.set('accountId', String(filters.accountId));
  if (filters.q?.trim()) params.set('q', filters.q.trim());
  return request<TransactionListResponse>(`/transactions?${params.toString()}`);
}

export function createTransaction(payload: TransactionPayload) {
  return request<Transaction>('/transactions', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateTransaction(id: number, payload: TransactionPayload) {
  return request<Transaction>(`/transactions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteTransaction(id: number) {
  return request<{ ok: boolean }>(`/transactions/${id}`, {
    method: 'DELETE',
  });
}

export function parseAIText(text: string) {
  return request<{ result: AIParseResult }>('/ai/parse', {
    method: 'POST',
    body: JSON.stringify({ text }),
  });
}
