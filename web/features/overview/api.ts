import { request } from '@/lib/api';
import type { Summary, TransactionListResponse } from '@/lib/types';

export function getOverviewSummary() {
  return request<Summary>('/reports/summary');
}

export function listRecentTransactions() {
  return request<TransactionListResponse>('/transactions?page=1&pageSize=5');
}
