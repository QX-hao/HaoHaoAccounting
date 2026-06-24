import AsyncStorage from '@react-native-async-storage/async-storage';

import { API_BASE } from '../config';
import type { ErrorResponse } from '../types/api';

const TOKEN_KEY = 'haohao_token';

type ApiErrorCode = ErrorResponse['code'] | 'network_error' | '';
type ApiErrorBody = Partial<ErrorResponse>;
const API_REQUEST_TIMEOUT_MS = 30_000;

export type ApiResponse<T> = {
  data: T;
  headers: Headers;
};

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  requestId: string;
  retryAfterSeconds: number | null;
  rateLimitLimit: number | null;
  rateLimitRemaining: number | null;
  rateLimitResetSeconds: number | null;
  authenticateChallenge: string;

  constructor(
    message: string,
    status: number,
    code: ApiErrorCode = '',
    requestId = '',
    retryAfterSeconds: number | null = null,
    rateLimitLimit: number | null = null,
    rateLimitRemaining: number | null = null,
    rateLimitResetSeconds: number | null = null,
    authenticateChallenge = '',
    cause?: unknown,
  ) {
    super(message, { cause });
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.retryAfterSeconds = retryAfterSeconds;
    this.rateLimitLimit = rateLimitLimit;
    this.rateLimitRemaining = rateLimitRemaining;
    this.rateLimitResetSeconds = rateLimitResetSeconds;
    this.authenticateChallenge = authenticateChallenge;
  }
}

export type DownloadResult = {
  bytes: Uint8Array;
  filename: string;
  contentType: string;
};

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
  const response = await requestWithResponse<T>(path, init);
  return response.data;
}

export async function requestWithResponse<T>(path: string, init: RequestInit = {}): Promise<ApiResponse<T>> {
  const headers = new Headers(init.headers || {});
  const token = await getToken();

  ensureRequestId(headers);
  headers.set('Accept', headers.get('Accept') || 'application/json');
  if (init.body !== undefined && init.body !== null && !(init.body instanceof FormData)) {
    // FormData 上传需要运行时自动生成 multipart boundary，手动设置会破坏上传体。
    headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetchAPI(path, {
    ...init,
    headers,
  });

  if (!resp.ok) {
    if (resp.status === 401) {
      // 移动端没有统一路由守卫，收到 401 时先清 token，让页面回到未登录态。
      await clearToken();
    }
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  const data = await resp.json().catch(() => ({}));
  return { data: data as T, headers: resp.headers };
}

export function upload<T>(path: string, formData: FormData) {
  return request<T>(path, {
    method: 'POST',
    body: formData,
  });
}

export function uploadWithResponse<T>(path: string, formData: FormData) {
  return requestWithResponse<T>(path, {
    method: 'POST',
    body: formData,
  });
}

export async function downloadText(path: string, accept = 'text/plain'): Promise<string> {
  const resp = await downloadResponse(path, accept);
  return resp.text();
}

export async function download(path: string, accept = '*/*'): Promise<DownloadResult> {
  const resp = await downloadResponse(path, accept);
  return {
    bytes: new Uint8Array(await resp.arrayBuffer()),
    filename: filenameFromDisposition(resp.headers.get('Content-Disposition')),
    contentType: resp.headers.get('Content-Type') || accept,
  };
}

async function downloadResponse(path: string, accept: string) {
  const headers = new Headers();
  ensureRequestId(headers);
  headers.set('Accept', accept);

  const token = await getToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetchAPI(path, {
    headers,
  });
  if (!resp.ok) {
    if (resp.status === 401) {
      await clearToken();
    }
    const data = await parseErrorBody(resp);
    throw apiError(resp, data);
  }
  return resp;
}

export async function logout() {
  const token = await getToken();
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
  return `mobile-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

async function parseErrorBody(resp: Response): Promise<ApiErrorBody> {
  const contentType = resp.headers.get('Content-Type') || '';
  if (isJSONContentType(contentType)) {
    return resp.json().catch(() => ({}));
  }
  const text = await resp.text().catch(() => '');
  return text ? { error: text } : {};
}

function isJSONContentType(contentType: string) {
  const mediaType = contentType.split(';', 1)[0]?.trim().toLowerCase() || '';
  // 兼容 application/*+json，确保结构化错误体能被解析出 code/requestId。
  return mediaType === 'application/json' || mediaType.startsWith('application/') && mediaType.endsWith('+json');
}

async function fetchAPI(path: string, init: RequestInit = {}) {
  const { signal, cleanup } = requestSignal(init.signal);
  try {
    return await fetch(`${API_BASE}${path}`, { ...init, signal });
  } catch (err) {
    throw networkError(err);
  } finally {
    cleanup();
  }
}

function requestSignal(callerSignal?: AbortSignal | null) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), API_REQUEST_TIMEOUT_MS);
  const abort = () => controller.abort();
  if (callerSignal) {
    if (callerSignal.aborted) {
      controller.abort();
    } else {
      callerSignal.addEventListener('abort', abort, { once: true });
    }
  }
  return {
    signal: controller.signal,
    cleanup: () => {
      // 清理定时器和外部 signal 监听，避免频繁请求后残留回调。
      clearTimeout(timeout);
      callerSignal?.removeEventListener('abort', abort);
    },
  };
}

function networkError(err: unknown) {
  const message = err instanceof Error && err.message ? err.message : '网络请求失败';
  return new ApiError(message, 0, 'network_error', '', null, null, null, null, '', err);
}

function apiError(resp: Response, data: ApiErrorBody): ApiError {
  const requestId = data.requestId || resp.headers.get('X-Request-ID') || '';
  return new ApiError(
    data.error || '请求失败',
    responseStatus(resp, data),
    data.code || '',
    requestId,
    retryAfterSeconds(resp),
    nonNegativeIntegerHeader(resp, 'RateLimit-Limit'),
    nonNegativeIntegerHeader(resp, 'RateLimit-Remaining'),
    nonNegativeIntegerHeader(resp, 'RateLimit-Reset'),
    resp.headers.get('WWW-Authenticate') || '',
  );
}

function responseStatus(resp: Response, data: ApiErrorBody) {
  const status = data.status;
  return typeof status === 'number' && Number.isInteger(status) && status >= 100 && status <= 599 ? status : resp.status;
}

function nonNegativeIntegerHeader(resp: Response, name: string): number | null {
  const value = resp.headers.get(name)?.trim();
  if (!value) return null;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) return null;
  return parsed;
}

function retryAfterSeconds(resp: Response): number | null {
  const value = resp.headers.get('Retry-After')?.trim();
  if (!value) return null;
  if (/^\d+$/.test(value)) {
    const seconds = Number(value);
    return Number.isSafeInteger(seconds) ? seconds : null;
  }
  if (/^[+-]?\d/.test(value)) {
    // 非整数秒数不符合 Retry-After 语义，也不要让 Date.parse 猜测。
    return null;
  }
  const retryAt = Date.parse(value);
  if (!Number.isFinite(retryAt)) return null;
  return Math.max(0, Math.ceil((retryAt - Date.now()) / 1000));
}

function filenameFromDisposition(disposition: string | null) {
  if (!disposition) return '';
  const params = contentDispositionParams(disposition);
  const extendedFilename = params.get('filename*');
  if (extendedFilename) {
    const filename = decodeExtendedFilename(extendedFilename);
    if (filename) {
      return filename;
    }
  }
  return params.get('filename') || '';
}

function contentDispositionParams(disposition: string) {
  const params = new Map<string, string>();
  for (const part of splitHeaderParameters(disposition).slice(1)) {
    const separator = part.indexOf('=');
    if (separator === -1) continue;
    const key = part.slice(0, separator).trim().toLowerCase();
    const value = part.slice(separator + 1).trim();
    if (key) {
      params.set(key, unquoteHeaderValue(value));
    }
  }
  return params;
}

function splitHeaderParameters(value: string) {
  const parts: string[] = [];
  let current = '';
  let quoted = false;
  let escaped = false;
  for (const char of value) {
    if (escaped) {
      current += char;
      escaped = false;
      continue;
    }
    if (char === '\\' && quoted) {
      current += char;
      escaped = true;
      continue;
    }
    if (char === '"') {
      quoted = !quoted;
    }
    if (char === ';' && !quoted) {
      parts.push(current.trim());
      current = '';
      continue;
    }
    current += char;
  }
  parts.push(current.trim());
  return parts;
}

function unquoteHeaderValue(value: string) {
  if (!value.startsWith('"') || !value.endsWith('"')) {
    return value;
  }
  let result = '';
  for (let index = 1; index < value.length - 1; index++) {
    if (value[index] === '\\' && index + 1 < value.length - 1) {
      index++;
    }
    result += value[index];
  }
  return result;
}

function decodeExtendedFilename(value: string) {
  const match = value.match(/^([^']*)'[^']*'(.*)$/);
  if (!match) {
    return safeDecodeURIComponent(value);
  }
  if (match[1] && match[1].toLowerCase() !== 'utf-8') {
    return '';
  }
  return safeDecodeURIComponent(match[2]);
}

function safeDecodeURIComponent(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return '';
  }
}
