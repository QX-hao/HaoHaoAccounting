'use client';

import { forwardRef, type FormEvent } from 'react';
import type { Account, Category, TransactionType } from '@/lib/types';
import { transactionTypeLabel } from '@/lib/format';

type Props = {
  accounts: Account[];
  filteredCategories: Category[];
  type: TransactionType;
  amount: string;
  categoryId: number;
  accountId: number;
  note: string;
  tags: string;
  occurredAt: string;
  onTypeChange: (value: TransactionType) => void;
  onAmountChange: (value: string) => void;
  onCategoryChange: (value: number) => void;
  onAccountChange: (value: number) => void;
  onNoteChange: (value: string) => void;
  onTagsChange: (value: string) => void;
  onOccurredAtChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
  disabled: boolean;
};

export const TransactionForm = forwardRef<HTMLInputElement, Props>(function TransactionForm({
  accounts,
  filteredCategories,
  type,
  amount,
  categoryId,
  accountId,
  note,
  tags,
  occurredAt,
  onTypeChange,
  onAmountChange,
  onCategoryChange,
  onAccountChange,
  onNoteChange,
  onTagsChange,
  onOccurredAtChange,
  onSubmit,
  disabled,
}, amountInputRef) {
  const canSubmit = !disabled && accounts.length > 0 && filteredCategories.length > 0;

  return (
    <form className="card grid" onSubmit={onSubmit}>
      <div className="hero-topline">
        <div>
          <span className="eyebrow">Manual</span>
          <h3>手动记账</h3>
        </div>
        <span className={`pill ${type}`}>{transactionTypeLabel(type)}</span>
      </div>

      <div className="amount-input">
        <label>金额</label>
        <input
          ref={amountInputRef}
          type="number"
          step="0.01"
          min="0.01"
          value={amount}
          onChange={(e) => onAmountChange(e.target.value)}
          placeholder="0.00"
          disabled={disabled}
          required
        />
      </div>

      <div className="type-toggle">
        {(['expense', 'income'] as const).map((item) => (
          <button
            className={type === item ? 'active' : ''}
            disabled={disabled}
            key={item}
            onClick={() => onTypeChange(item)}
            type="button"
          >
            {transactionTypeLabel(item)}
          </button>
        ))}
      </div>

      <div className="category-grid">
        {filteredCategories.length === 0 ? <div className="empty-state">暂无可用分类，请先创建该类型分类。</div> : null}
        {filteredCategories.map((item) => (
          <button
            className={categoryId === item.id ? 'category-choice active' : 'category-choice'}
            disabled={disabled}
            key={item.id}
            onClick={() => onCategoryChange(item.id)}
            type="button"
          >
            <span className="category-dot" aria-hidden="true">
              {item.name.slice(0, 1)}
            </span>
            <span>{item.name}</span>
          </button>
        ))}
      </div>

      <div className="form-grid">
        <select value={accountId} onChange={(e) => onAccountChange(Number(e.target.value))} disabled={disabled || accounts.length === 0} required>
          {accounts.length === 0 ? <option value={0}>暂无账户，请先创建账户</option> : null}
          {accounts.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
        <input type="datetime-local" value={occurredAt} onChange={(e) => onOccurredAtChange(e.target.value)} disabled={disabled} required />
        <input className="field-full" value={note} onChange={(e) => onNoteChange(e.target.value)} placeholder="备注" disabled={disabled} required />
        <input className="field-full" value={tags} onChange={(e) => onTagsChange(e.target.value)} placeholder="标签，用逗号分隔" disabled={disabled} />
      </div>
      <button className="primary" disabled={!canSubmit} type="submit">
        {disabled ? '保存中...' : '保存账单'}
      </button>
    </form>
  );
});
