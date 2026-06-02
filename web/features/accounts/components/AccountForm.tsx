'use client';

import type { FormEvent } from 'react';

type Props = {
  name: string;
  type: string;
  submitLabel?: string;
  onNameChange: (value: string) => void;
  onTypeChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
};

export function AccountForm({ name, type, submitLabel = '新增账户', onNameChange, onTypeChange, onSubmit }: Props) {
  return (
    <form className="card toolbar" onSubmit={onSubmit}>
      <input value={name} onChange={(e) => onNameChange(e.target.value)} placeholder="账户名称" required />
      <select value={type} onChange={(e) => onTypeChange(e.target.value)}>
        <option value="cash">现金</option>
        <option value="bank">银行卡</option>
        <option value="alipay">支付宝</option>
        <option value="wechat">微信</option>
        <option value="custom">自定义</option>
      </select>
      <button className="primary" type="submit">
        {submitLabel}
      </button>
    </form>
  );
}
