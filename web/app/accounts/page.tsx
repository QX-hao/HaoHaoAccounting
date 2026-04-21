'use client';

import { FormEvent, useEffect, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';
import { Account } from '@/lib/types';

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState('custom');
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      const rows = await request<Account[]>('/accounts');
      setAccounts(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function createAccount(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await request('/accounts', {
        method: 'POST',
        body: JSON.stringify({ name, type, balance: 0 }),
      });
      setName('');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败');
    }
  }

  return (
    <PageFrame title="账户管理" subtitle="维护现金、银行卡、支付宝、微信等账户">
      {error ? <div className="error">{error}</div> : null}

      <form className="card toolbar" onSubmit={createAccount}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="账户名称" required />
        <select value={type} onChange={(e) => setType(e.target.value)}>
          <option value="cash">现金</option>
          <option value="bank">银行卡</option>
          <option value="alipay">支付宝</option>
          <option value="wechat">微信</option>
          <option value="custom">自定义</option>
        </select>
        <button className="primary" type="submit">
          新增账户
        </button>
      </form>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>账户名</th>
              <th>类型</th>
              <th>余额</th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((item) => (
              <tr key={item.id}>
                <td>{item.name}</td>
                <td>{item.type}</td>
                <td>¥ {item.balance.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
