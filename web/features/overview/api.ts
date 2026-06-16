import { api } from '@/shared/api';

export function getOverviewSummary() {
  return api.reports.getReportsSummary();
}

export function listRecentTransactions() {
  return api.transactions.getTransactions({ page: 1, pageSize: 5 });
}
