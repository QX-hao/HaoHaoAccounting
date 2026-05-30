import { request } from '@/lib/api';
import type { Summary } from '@/lib/types';

export function getReportSummary() {
  return request<Summary>('/reports/summary');
}
