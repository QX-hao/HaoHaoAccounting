import { request } from '../../shared/api/client';
import type { AIParseResult, TransactionType } from '../../shared/types/accounting';

export function createTransaction(payload: {
  type: TransactionType;
  amount: number;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string[];
  source: string;
  occurredAt: string;
}) {
  return request('/transactions', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function parseAIText(text: string) {
  return request<{ result: AIParseResult }>('/ai/parse', {
    method: 'POST',
    body: JSON.stringify({ text }),
  });
}
