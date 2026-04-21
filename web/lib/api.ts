import { getToken } from './auth';
import { API_BASE } from './config';

export async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers || {});
  headers.set('Content-Type', headers.get('Content-Type') || 'application/json');

  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  });

  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data as T;
}

export async function upload<T>(path: string, formData: FormData): Promise<T> {
  const headers = new Headers();
  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: formData,
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data as T;
}

export function exportUrl(path: string): string {
  const token = getToken();
  const prefix = `${API_BASE}${path}`;
  if (!token) return prefix;

  return `data:text/plain,请先登录后通过前端发起导出（浏览器直链无法自动附带Bearer Token）`;
}
