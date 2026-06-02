'use client';

import type { FormEvent } from 'react';
import type { TransactionType } from '@/lib/types';

type Props = {
  name: string;
  type: TransactionType;
  submitLabel?: string;
  onNameChange: (value: string) => void;
  onTypeChange: (value: TransactionType) => void;
  onSubmit: (event: FormEvent) => void;
};

export function CategoryForm({ name, type, submitLabel = '新增分类', onNameChange, onTypeChange, onSubmit }: Props) {
  return (
    <form className="card toolbar" onSubmit={onSubmit}>
      <input value={name} onChange={(e) => onNameChange(e.target.value)} placeholder="分类名称" required />
      <select value={type} onChange={(e) => onTypeChange(e.target.value as TransactionType)}>
        <option value="expense">支出</option>
        <option value="income">收入</option>
      </select>
      <button className="primary" type="submit">
        {submitLabel}
      </button>
    </form>
  );
}
