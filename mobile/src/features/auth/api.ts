import { request } from '../../shared/api/client';

export function login(payload: { username: string; password: string }) {
  return request<{ token: string }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function verifyCurrentUser() {
  return request('/me');
}
