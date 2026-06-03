import { api } from '../../shared/api';

export async function loadDashboardData() {
  const [summary, transactions, accounts, categories] = await Promise.all([
    api.reports.getReportsSummary({}),
    api.transactions.getTransactions({ page: 1, pageSize: 20 }),
    api.accounts.getAccounts(),
    api.categories.getCategories({}),
  ]);

  return {
    summary,
    transactions: transactions.items || [],
    transactionTotal: transactions.pagination?.total ?? transactions.items?.length ?? 0,
    accounts,
    categories,
  };
}
