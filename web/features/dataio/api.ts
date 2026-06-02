import { download, upload } from '@/lib/api';
import type { ImportPreview, ImportResult } from '@/lib/types';

export type ExportFormat = 'csv' | 'xlsx';

export function exportTransactions(format: ExportFormat) {
  return download(`/io/export?format=${format}`);
}

export function importTransactions(formData: FormData) {
  return upload<ImportResult>('/io/import', formData);
}

export function previewImport(formData: FormData) {
  return upload<ImportPreview>('/io/import/preview', formData);
}
