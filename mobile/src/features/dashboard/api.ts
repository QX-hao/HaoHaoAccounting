import { request } from '../../shared/api/client';
import type { Account, Category, Summary, Transaction } from '../../shared/types/accounting';

export async function loadDashboardData() {
  const [summary, transactions, accounts, categories] = await Promise.all([
    request<Summary>('/reports/summary'),
    request<{ items: Transaction[] }>('/transactions?page=1&pageSize=20'),
    request<Account[]>('/accounts'),
    request<Category[]>('/categories'),
  ]);

  return {
    summary,
    transactions: transactions.items || [],
    accounts,
    categories,
  };
}
