import { API_BASE } from '@/lib/config';
import type { ErrorResponse } from '@/shared/types/api';
import { clearToken, getToken } from '@/shared/auth/token';

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

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await requestWithResponse<T>(path, init);
  return response.data;
}

export async function requestWithResponse<T>(path: string, init: RequestInit = {}): Promise<ApiResponse<T>> {
  const headers = new Headers(init.headers || {});
  ensureRequestId(headers);
  headers.set('Accept', headers.get('Accept') || 'application/json');
  if (init.body !== undefined && init.body !== null && !(init.body instanceof FormData)) {
    // FormData 让浏览器自己补 boundary；普通 JSON 请求才设置 Content-Type。
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
  return { data: data as T, headers: resp.headers };
}

export async function upload<T>(path: string, formData: FormData): Promise<T> {
  const response = await uploadWithResponse<T>(path, formData);
  return response.data;
}

export async function uploadWithResponse<T>(path: string, formData: FormData): Promise<ApiResponse<T>> {
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
  return { data: data as T, headers: resp.headers };
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
  // 任意接口确认 token 失效后，前端立即清本地 token，避免后续请求继续带过期凭据。
  clearToken();
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
  }
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
  // 后端可能返回 application/problem+json 一类结构化 JSON，不能只认 application/json。
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
      // 每次请求结束都移除监听和定时器，避免页面长时间使用后积累无效 abort 回调。
      clearTimeout(timeout);
      callerSignal?.removeEventListener('abort', abort);
    },
  };
}

function networkError(err: unknown) {
  const message = err instanceof Error && err.message ? err.message : 'Network request failed';
  return new ApiError(message, 0, 'network_error', '', null, null, null, null, '', err);
}

function apiError(resp: Response, data: ApiErrorBody): ApiError {
  const requestId = data.requestId || resp.headers.get('X-Request-ID') || '';
  return new ApiError(
    data.error || 'Request failed',
    resp.status,
    data.code || '',
    requestId,
    retryAfterSeconds(resp),
    nonNegativeIntegerHeader(resp, 'RateLimit-Limit'),
    nonNegativeIntegerHeader(resp, 'RateLimit-Remaining'),
    nonNegativeIntegerHeader(resp, 'RateLimit-Reset'),
    resp.headers.get('WWW-Authenticate') || '',
  );
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
    // RFC 允许 HTTP-date；带符号或小数的数字既不是合法秒数，也不要交给 Date.parse 误判。
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
      // Content-Disposition 的 filename 可能包含引号内分号，只能在非引号状态切分。
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
    // 只接受 UTF-8 的 RFC 5987 filename*，避免错误字符集解码出乱码文件名。
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
