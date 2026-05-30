export function formatMoney(value?: number | null) {
  return `¥ ${(value ?? 0).toFixed(2)}`;
}

export function transactionTypeLabel(type: 'income' | 'expense') {
  return type === 'income' ? '收入' : '支出';
}
