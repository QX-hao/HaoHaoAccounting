import { download, upload } from '@/lib/api';
import { api } from '@/shared/api';

export type ExportFormat = 'csv' | 'xlsx';

export function exportTransactions(format: ExportFormat) {
  return download(`/io/export?format=${format}`, exportAccept(format));
}

function exportAccept(format: ExportFormat) {
  if (format === 'xlsx') {
    return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  }
  return 'text/csv';
}

export function importTransactions(formData: FormData) {
  return api.dataio.postIoImportJobs(formData);
}

export function previewImport(formData: FormData) {
  return api.dataio.postIoImportPreview(formData);
}

export function listImportJobs() {
  return api.dataio.getIoImportJobs();
}

export function getImportJob(id: number) {
  return api.dataio.getIoImportJobsById({ id });
}
