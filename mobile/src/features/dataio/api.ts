import { api } from '../../shared/api';
import { API_BASE, getToken, request } from '../../shared/api/client';
import type { ImportJob, ImportPreview, ImportResult } from '../../shared/types/accounting';

export type ImportFileAsset = {
  uri: string;
  name: string;
  mimeType?: string | null;
};

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

export async function exportCSVText() {
  const token = await getToken();
  const resp = await fetch(`${API_BASE}/io/export?format=csv`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  const text = await resp.text();
  if (!resp.ok) {
    throw new Error(text || '导出失败');
  }
  return text;
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
