import { api } from '../../shared/api';
import { download, downloadText } from '../../shared/api/client';
import type { ImportJob, ImportPreview, ImportResult } from '../../shared/types/accounting';

export type ImportFileAsset = {
  uri: string;
  name: string;
  mimeType?: string | null;
};

export type ExportFormat = 'csv' | 'xlsx';

export function previewImportText(content: string) {
  return api.dataio.postIoImportTextPreview({ filename: 'mobile-import.csv', content });
}

export function importText(content: string) {
  return api.dataio.postIoImportText({ filename: 'mobile-import.csv', content });
}

export function previewImportFile(asset: ImportFileAsset) {
  return api.dataio.postIoImportPreview(importFileFormData(asset));
}

export function startImportFileJob(asset: ImportFileAsset) {
  return api.dataio.postIoImportJobs(importFileFormData(asset));
}

export function exportCSVText() {
  return downloadText('/io/export?format=csv', 'text/csv');
}

export function exportTransactionsFile(format: ExportFormat) {
  return download(`/io/export?format=${format}`, exportAccept(format));
}

function exportAccept(format: ExportFormat) {
  if (format === 'xlsx') {
    return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  }
  return 'text/csv';
}

function importFileFormData(asset: ImportFileAsset) {
  const formData = new FormData();
  const filePart = {
    uri: asset.uri,
    name: asset.name || 'mobile-import.csv',
    type: asset.mimeType || mimeTypeFromName(asset.name),
  };
  formData.append('file', filePart as unknown as Blob);
  return formData;
}

function mimeTypeFromName(name: string) {
  const lower = name.toLowerCase();
  if (lower.endsWith('.xlsx')) {
    return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  }
  if (lower.endsWith('.xls')) {
    return 'application/vnd.ms-excel';
  }
  return 'text/csv';
}
