import { api } from '../../shared/api';

export function login(payload: { username: string; password: string }) {
  return api.auth.postAuthLogin(payload);
}

export function verifyCurrentUser() {
  return api.auth.getMe();
}
