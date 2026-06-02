import { request } from '../../shared/api/client';
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
  const params = new URLSearchParams({
    page: String(filters.page),
    pageSize: String(filters.pageSize),
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
