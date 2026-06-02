'use client';

import { FormEvent, useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Account } from '@/lib/types';
import { createAccount, deleteAccount, listAccounts, updateAccount } from './api';
import { AccountForm } from './components/AccountForm';
import { AccountGrid } from './components/AccountGrid';

export default function AccountsFeaturePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState('custom');
  const [editing, setEditing] = useState<Account | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);

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
    setNotice('');
    try {
      setBusy(true);
      if (editing) {
        await updateAccount(editing.id, { name, type, balance: editing.balance });
        setNotice('账户已更新');
      } else {
        await createAccount({ name, type, balance: 0 });
        setNotice('账户已创建');
      }
      resetForm();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : editing ? '更新失败' : '创建失败');
    } finally {
      setBusy(false);
    }
  }

  function startEdit(account: Account) {
    setEditing(account);
    setName(account.name);
    setType(account.type);
    setError('');
    setNotice('');
  }

  async function remove(account: Account) {
    const ok = window.confirm(`确定删除账户「${account.name}」吗？已有账单使用的账户不能删除。`);
    if (!ok) return;
    setError('');
    setNotice('');
    try {
      setBusy(true);
      await deleteAccount(account.id);
      if (editing?.id === account.id) resetForm();
      setNotice('账户已删除');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    } finally {
      setBusy(false);
    }
  }

  function resetForm() {
    setEditing(null);
      setName('');
    setType('custom');
  }

  return (
    <PageFrame title="账户" subtitle="维护现金、银行卡、支付宝、微信等账户。">
      {error ? <div className="error">{error}</div> : null}
      {notice ? <div className="success">{notice}</div> : null}
      <AccountForm name={name} type={type} submitLabel={editing ? '保存修改' : '新增账户'} onNameChange={setName} onTypeChange={setType} onSubmit={submit} />
      {editing ? (
        <div className="toolbar">
          <span className="badge">正在编辑：{editing.name}</span>
          <button className="ghost" type="button" disabled={busy} onClick={resetForm}>
            取消编辑
          </button>
        </div>
      ) : null}
      <AccountGrid accounts={accounts} disabled={busy} onEdit={startEdit} onDelete={remove} />
    </PageFrame>
  );
}
