import { API_BASE, getToken, request } from '../../shared/api/client';
import type { ImportPreview, ImportResult } from '../../shared/types/accounting';

export function previewImportText(content: string) {
  return request<ImportPreview>('/io/import/text/preview', {
    method: 'POST',
    body: JSON.stringify({ filename: 'mobile-import.csv', content }),
  });
}

export function importText(content: string) {
  return request<ImportResult>('/io/import/text', {
    method: 'POST',
    body: JSON.stringify({ filename: 'mobile-import.csv', content }),
  });
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
