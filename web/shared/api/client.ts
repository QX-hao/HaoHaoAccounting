import { API_BASE } from '@/lib/config';
import { clearToken, getToken } from '@/shared/auth/token';

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
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
    handleUnauthorized(resp.status);
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
    handleUnauthorized(resp.status);
    throw new Error(data.error || 'Request failed');
  }
  return data as T;
}

export async function download(path: string): Promise<Blob> {
  const token = getToken();
  const resp = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!resp.ok) {
    handleUnauthorized(resp.status);
    const text = await resp.text();
    throw new Error(text || 'Request failed');
  }
  return resp.blob();
}

function handleUnauthorized(status: number) {
  if (status !== 401 || typeof window === 'undefined') return;
  clearToken();
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
  }
}
