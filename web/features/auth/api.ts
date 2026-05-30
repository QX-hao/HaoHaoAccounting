import { request } from '@/lib/api';

export function login(payload: { username: string; password: string }) {
  return request<{ token: string; user: unknown }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function verifyCurrentUser() {
  return request('/me');
}
