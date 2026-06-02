import { request } from '../../shared/api/client';
import type { Account, Category, Summary, TransactionListResponse } from '../../shared/types/accounting';

export async function loadDashboardData() {
  const [summary, transactions, accounts, categories] = await Promise.all([
    request<Summary>('/reports/summary'),
    request<TransactionListResponse>('/transactions?page=1&pageSize=20'),
    request<Account[]>('/accounts'),
    request<Category[]>('/categories'),
  ]);

  return {
    summary,
    transactions: transactions.items || [],
    transactionTotal: transactions.pagination?.total ?? transactions.items?.length ?? 0,
    accounts,
    categories,
  };
}
