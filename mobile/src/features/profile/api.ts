import { request } from '../../shared/api/client';

export function createSimpleCategory(name: string) {
  return request('/categories', {
    method: 'POST',
    body: JSON.stringify({ name, type: 'expense' }),
  });
}

export function createSimpleAccount(name: string) {
  return request('/accounts', {
    method: 'POST',
    body: JSON.stringify({ name, type: 'custom', balance: 0 }),
  });
}
