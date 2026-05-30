'use client';

import { FormEvent, useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Account } from '@/lib/types';
import { createAccount, listAccounts } from './api';
import { AccountForm } from './components/AccountForm';
import { AccountGrid } from './components/AccountGrid';

export default function AccountsFeaturePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState('custom');
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      setAccounts(await listAccounts());
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await createAccount({ name, type, balance: 0 });
      setName('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败');
    }
  }

  return (
    <PageFrame title="账户" subtitle="维护现金、银行卡、支付宝、微信等账户。">
      {error ? <div className="error">{error}</div> : null}
      <AccountForm name={name} type={type} onNameChange={setName} onTypeChange={setType} onSubmit={submit} />
      <AccountGrid accounts={accounts} />
    </PageFrame>
  );
}
