import { api } from '@/shared/api';

export function getCurrentUser() {
  return api.me.getMe();
}
