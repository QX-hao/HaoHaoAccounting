import { download, upload } from '@/lib/api';

export type ExportFormat = 'csv' | 'xlsx';

export function exportTransactions(format: ExportFormat) {
  return download(`/io/export?format=${format}`);
}

export function importTransactions(formData: FormData) {
  return upload<{ success: number; failed: number; errors: string[] }>('/io/import', formData);
}
