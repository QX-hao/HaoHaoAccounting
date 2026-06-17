import { API_BASE } from '@/lib/config';
import type { ErrorResponse } from '@/shared/types/api';
import { clearToken, getToken } from '@/shared/auth/token';

type ApiErrorCode = ErrorResponse['code'] | 'network_error' | '';
type ApiErrorBody = Partial<ErrorResponse>;

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  requestId: string;
  retryAfterSeconds: number | null;
  authenticateChallenge: string;

  constructor(
    message: string,
    status: number,
    code: ApiErrorCode = '',
    requestId = '',
    retryAfterSeconds: number | null = null,
    authenticateChallenge = '',
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.retryAfterSeconds = retryAfterSeconds;
    this.authenticateChallenge = authenticateChallenge;
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  ensureRequestId(headers);
  headers.set('Accept', headers.get('Accept') || 'application/json');
  if (init.body !== undefined && init.body !== null && !(init.body instanceof FormData)) {
    headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
  }

  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetchAPI(path, {
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
  ensureRequestId(headers);
  headers.set('Accept', 'application/json');
  const token = getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetchAPI(path, {
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
  ensureRequestId(headers);
  headers.set('Accept', accept);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const resp = await fetchAPI(path, {
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

  await fetchAPI('/auth/logout', {
    method: 'POST',
    headers: withRequestId({ Accept: 'application/json', Authorization: `Bearer ${token}` }),
  }).catch(() => undefined);
}

function ensureRequestId(headers: Headers) {
  if (!headers.has('X-Request-ID')) {
    headers.set('X-Request-ID', newRequestId());
  }
}

function withRequestId(headers: HeadersInit): Headers {
  const next = new Headers(headers);
  ensureRequestId(next);
  return next;
}

function newRequestId() {
  return `web-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
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

async function fetchAPI(path: string, init: RequestInit = {}) {
  try {
    return await fetch(`${API_BASE}${path}`, init);
  } catch (err) {
    throw networkError(err);
  }
}

function networkError(err: unknown) {
  const message = err instanceof Error && err.message ? err.message : 'Network request failed';
  return new ApiError(message, 0, 'network_error');
}

function apiError(resp: Response, data: ApiErrorBody): ApiError {
  const requestId = data.requestId || resp.headers.get('X-Request-ID') || '';
  return new ApiError(
    data.error || 'Request failed',
    resp.status,
    data.code || '',
    requestId,
    retryAfterSeconds(resp),
    resp.headers.get('WWW-Authenticate') || '',
  );
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
    const filename = safeDecodeURIComponent(utf8Match[1]);
    if (filename) return filename;
  }
  const asciiMatch = disposition.match(/filename="?([^";]+)"?/i);
  return asciiMatch?.[1] || '';
}

function safeDecodeURIComponent(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return '';
  }
}
