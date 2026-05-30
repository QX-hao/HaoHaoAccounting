import { request } from '@/lib/api';
import type { CurrentUser } from '@/lib/types';

export function getCurrentUser() {
  return request<CurrentUser>('/me');
}
