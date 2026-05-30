import { request } from '@/lib/api';
import type { Category, TransactionType } from '@/lib/types';

export function listCategories() {
  return request<Category[]>('/categories');
}

export function createCategory(payload: { name: string; type: TransactionType }) {
  return request<Category>('/categories', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
