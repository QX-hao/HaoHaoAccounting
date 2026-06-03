import { api } from '@/shared/api';

export function listAccounts() {
  return api.accounts.getAccounts();
}

export function createAccount(payload: { name: string; type: string; balance: number }) {
  return api.accounts.postAccounts(payload);
}

export function updateAccount(id: number, payload: { name: string; type: string; balance: number }) {
  return api.accounts.putAccountsById({ id }, payload);
}

export function deleteAccount(id: number) {
  return api.accounts.deleteAccountsById({ id });
}
