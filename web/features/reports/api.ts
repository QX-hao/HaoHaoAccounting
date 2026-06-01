import { request } from '@/lib/api';
import type { Summary } from '@/lib/types';

export type ReportSummaryParams = {
  start?: string;
  end?: string;
  categoryId?: number;
  accountId?: number;
};

export function getReportSummary(params: ReportSummaryParams = {}) {
  const search = new URLSearchParams();
  if (params.start) search.set('start', params.start);
  if (params.end) search.set('end', params.end);
  if (params.categoryId) search.set('categoryId', String(params.categoryId));
  if (params.accountId) search.set('accountId', String(params.accountId));
  const query = search.toString();
  return request<Summary>(`/reports/summary${query ? `?${query}` : ''}`);
}
