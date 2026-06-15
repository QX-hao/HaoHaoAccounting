import { API_BASE } from '@/lib/config';
import type { ErrorResponse } from '@/shared/types/api';
import { clearToken, getToken } from '@/shared/auth/token';

type ApiErrorCode = ErrorResponse['code'] | '';
type ApiErrorBody = Partial<ErrorResponse>;

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  requestId: string;
  retryAfterSeconds: number | null;

  constructor(message: string, status: number, code: ApiErrorCode = '', requestId = '', retryAfterSeconds: number | null = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  headers.set('Accept', headers.get('Accept') || 'application/json');
  if (init.body !== undefined && init.body !== null && !(init.body instanceof FormData)) {
    headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
  }

  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  });

  if (!resp.ok) {
    handleUnauthorized(resp.status);
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  const data = await resp.json().catch(() => ({}));
  return data as T;
}

export async function upload<T>(path: string, formData: FormData): Promise<T> {
  const headers = new Headers();
  headers.set('Accept', 'application/json');
  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: formData,
  });
  if (!resp.ok) {
    handleUnauthorized(resp.status);
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  const data = await resp.json().catch(() => ({}));
  return data as T;
}

export type DownloadResult = {
  blob: Blob;
  filename: string;
};

export async function download(path: string, accept = '*/*'): Promise<DownloadResult> {
  const token = getToken();
  const headers = new Headers();
  headers.set('Accept', accept);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const resp = await fetch(`${API_BASE}${path}`, {
    headers,
  });
  if (!resp.ok) {
    handleUnauthorized(resp.status);
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  return {
    blob: await resp.blob(),
    filename: filenameFromDisposition(resp.headers.get('Content-Disposition')) || 'download',
  };
}

export async function logout(): Promise<void> {
  const token = getToken();
  if (!token) return;

  await fetch(`${API_BASE}/auth/logout`, {
    method: 'POST',
    headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
  }).catch(() => undefined);
}

function handleUnauthorized(status: number) {
  if (status !== 401 || typeof window === 'undefined') return;
  clearToken();
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
  }
}

async function parseErrorBody(resp: Response): Promise<ApiErrorBody> {
  const contentType = resp.headers.get('Content-Type') || '';
  if (contentType.includes('application/json')) {
    return resp.json().catch(() => ({}));
  }
  const text = await resp.text().catch(() => '');
  return text ? { error: text } : {};
}

function apiError(resp: Response, data: ApiErrorBody): ApiError {
  const requestId = data.requestId || resp.headers.get('X-Request-ID') || '';
  return new ApiError(data.error || 'Request failed', resp.status, data.code || '', requestId, retryAfterSeconds(resp));
}

function retryAfterSeconds(resp: Response): number | null {
  const value = resp.headers.get('Retry-After')?.trim();
  if (!value) return null;
  const seconds = Number(value);
  if (Number.isFinite(seconds)) {
    if (seconds < 0) return null;
    return Math.ceil(seconds);
  }
  const retryAt = Date.parse(value);
  if (!Number.isFinite(retryAt)) return null;
  return Math.max(0, Math.ceil((retryAt - Date.now()) / 1000));
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
