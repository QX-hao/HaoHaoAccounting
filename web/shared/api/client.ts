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

export type DownloadResult = {
  blob: Blob;
  filename: string;
};

export async function download(path: string): Promise<DownloadResult> {
  const token = getToken();
  const resp = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!resp.ok) {
    handleUnauthorized(resp.status);
    const text = await resp.text();
    throw new Error(text || 'Request failed');
  }
  return {
    blob: await resp.blob(),
    filename: filenameFromDisposition(resp.headers.get('Content-Disposition')) || 'download',
  };
}

function handleUnauthorized(status: number) {
  if (status !== 401 || typeof window === 'undefined') return;
  clearToken();
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
  }
}

function filenameFromDisposition(disposition: string | null) {
  if (!disposition) return '';
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1]);
  }
  const asciiMatch = disposition.match(/filename="?([^";]+)"?/i);
  return asciiMatch?.[1] || '';
}
