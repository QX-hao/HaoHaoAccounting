import { api } from '@/shared/api';
import type { Category, TransactionType } from '@/lib/types';

export function listCategories() {
  return api.categories.getCategories({});
}

export function createCategory(payload: { name: string; type: TransactionType }) {
  return api.categories.postCategories(payload);
}

export function updateCategory(id: number, payload: { name: string; type: TransactionType }) {
  return api.categories.putCategoriesById({ id }, payload);
}

export function deleteCategory(id: number) {
  return api.categories.deleteCategoriesById({ id });
}
