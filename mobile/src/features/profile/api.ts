import { api } from '../../shared/api';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';

export function createSimpleCategory(name: string) {
  return api.categories.postCategories({ name, type: 'expense' }) as Promise<Category>;
}

export function createSimpleAccount(name: string) {
  return api.accounts.postAccounts({ name, type: 'custom', balance: 0 }) as Promise<Account>;
}

export function createCategory(payload: { name: string; type: TransactionType }) {
  return api.categories.postCategories(payload) as Promise<Category>;
}

export function updateCategory(id: number, payload: { name: string; type: TransactionType }) {
  return api.categories.putCategoriesById({ id }, payload) as Promise<Category>;
}

export function deleteCategory(id: number) {
  return api.categories.deleteCategoriesById({ id });
}

export function createAccount(payload: { name: string; type: string; balance: number }) {
  return api.accounts.postAccounts(payload) as Promise<Account>;
}

export function updateAccount(id: number, payload: { name: string; type: string; balance: number }) {
  return api.accounts.putAccountsById({ id }, payload) as Promise<Account>;
}

export function deleteAccount(id: number) {
  return api.accounts.deleteAccountsById({ id });
}
