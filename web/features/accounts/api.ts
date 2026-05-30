import { request } from '@/lib/api';
import type { Account } from '@/lib/types';

export function listAccounts() {
  return request<Account[]>('/accounts');
}

export function createAccount(payload: { name: string; type: string; balance: number }) {
  return request<Account>('/accounts', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
