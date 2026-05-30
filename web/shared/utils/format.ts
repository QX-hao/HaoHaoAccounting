import type { TransactionType } from '@/shared/types/accounting';

export function formatMoney(value?: number | null, options: { signed?: boolean } = {}) {
  const amount = value ?? 0;
  const formatted = new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
  }).format(Math.abs(amount));

  if (!options.signed || amount === 0) return formatted;
  return `${amount > 0 ? '+' : '-'}${formatted}`;
}

export function formatDateTime(value?: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export function transactionTypeLabel(type: TransactionType) {
  return type === 'income' ? '收入' : '支出';
}
