import AsyncStorage from '@react-native-async-storage/async-storage';

import type { ErrorResponse } from '../types/api';

const TOKEN_KEY = 'haohao_token';
export const API_BASE = process.env.EXPO_PUBLIC_API_BASE || 'http://127.0.0.1:8080/api/v1';

type ApiErrorCode = ErrorResponse['code'] | '';
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

export async function getToken() {
  return (await AsyncStorage.getItem(TOKEN_KEY)) || '';
}

export async function setToken(token: string) {
  await AsyncStorage.setItem(TOKEN_KEY, token);
}

export async function clearToken() {
  await AsyncStorage.removeItem(TOKEN_KEY);
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  const token = await getToken();

  headers.set('Accept', headers.get('Accept') || 'application/json');
  if (init.body !== undefined && init.body !== null && !(init.body instanceof FormData)) {
    headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  });

  if (!resp.ok) {
    if (resp.status === 401) {
      await clearToken();
    }
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  const data = await resp.json().catch(() => ({}));
  return data as T;
}

export function upload<T>(path: string, formData: FormData) {
  return request<T>(path, {
    method: 'POST',
    body: formData,
  });
}

export async function downloadText(path: string, accept = 'text/plain'): Promise<string> {
  const headers = new Headers();
  headers.set('Accept', accept);

  const token = await getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    headers,
  });
  if (!resp.ok) {
    if (resp.status === 401) {
      await clearToken();
    }
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  return resp.text();
}

export async function logout() {
  const token = await getToken();
  if (!token) return;

  await fetch(`${API_BASE}/auth/logout`, {
    method: 'POST',
    headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
  }).catch(() => undefined);
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
  return new ApiError(
    data.error || '请求失败',
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
