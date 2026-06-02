import { request } from '../../shared/api/client';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';

export function createSimpleCategory(name: string) {
  return request<Category>('/categories', {
    method: 'POST',
    body: JSON.stringify({ name, type: 'expense' }),
  });
}

export function createSimpleAccount(name: string) {
  return request<Account>('/accounts', {
    method: 'POST',
    body: JSON.stringify({ name, type: 'custom', balance: 0 }),
  });
}

export function createCategory(payload: { name: string; type: TransactionType }) {
  return request<Category>('/categories', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateCategory(id: number, payload: { name: string; type: TransactionType }) {
  return request<Category>(`/categories/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteCategory(id: number) {
  return request<{ ok: boolean }>(`/categories/${id}`, {
    method: 'DELETE',
  });
}

export function createAccount(payload: { name: string; type: string; balance: number }) {
  return request<Account>('/accounts', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateAccount(id: number, payload: { name: string; type: string; balance: number }) {
  return request<Account>(`/accounts/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteAccount(id: number) {
  return request<{ ok: boolean }>(`/accounts/${id}`, {
    method: 'DELETE',
  });
}
