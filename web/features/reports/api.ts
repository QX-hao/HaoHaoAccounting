import { api } from '@/shared/api';

export type ReportSummaryParams = {
  start?: string;
  end?: string;
  categoryId?: number;
  accountId?: number;
  trend?: 'day' | 'week' | 'month';
};

export function getReportSummary(params: ReportSummaryParams = {}) {
  return api.reports.getReportsSummary(params);
}
