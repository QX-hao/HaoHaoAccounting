'use client';

import { FormEvent, useEffect, useMemo, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';
import { Account, Category, Transaction } from '@/lib/types';

type TxResponse = {
  items: Transaction[];
};

export default function TransactionsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const [type, setType] = useState<'expense' | 'income'>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState<number>(0);
  const [accountId, setAccountId] = useState<number>(0);
  const [note, setNote] = useState('');
  const [tags, setTags] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => new Date().toISOString().slice(0, 16));
  const [aiText, setAiText] = useState('今天午饭35');

  const filteredCategories = useMemo(
    () => categories.filter((c) => c.type === type),
    [categories, type],
  );

  async function loadData() {
    setError('');
    try {
      const [a, c, t] = await Promise.all([
        request<Account[]>('/accounts'),
        request<Category[]>('/categories'),
        request<TxResponse>('/transactions?page=1&pageSize=50'),
      ]);
      setAccounts(a);
      setCategories(c);
      setTransactions(t.items || []);

      if (a[0] && !accountId) setAccountId(a[0].id);
      const firstCategory = c.find((item) => item.type === type);
      if (firstCategory && !categoryId) setCategoryId(firstCategory.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    const firstCategory = filteredCategories[0];
    if (firstCategory) setCategoryId(firstCategory.id);
  }, [type]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setNotice('');
    try {
      await request('/transactions', {
        method: 'POST',
        body: JSON.stringify({
          type,
          amount: Number(amount),
          categoryId,
          accountId,
          note,
          tags: tags
            .split(',')
            .map((v) => v.trim())
            .filter(Boolean),
          occurredAt: new Date(occurredAt).toISOString(),
          source: 'manual',
        }),
      });
      setNotice('账单已保存');
      setAmount('');
      setNote('');
      setTags('');
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    }
  }

  async function runAIParse() {
    setError('');
    setNotice('');
    try {
      const resp = await request<{
        result: {
          type: 'income' | 'expense';
          amount: number;
          category: string;
          account: string;
          note: string;
          occurredAt: string;
        };
      }>('/ai/parse', {
        method: 'POST',
        body: JSON.stringify({ text: aiText }),
      });

      const result = resp.result;
      setType(result.type);
      setAmount(String(result.amount));
      setNote(result.note || aiText);
      setOccurredAt(new Date(result.occurredAt).toISOString().slice(0, 16));

      const matchedCategory = categories.find((c) => c.name === result.category && c.type === result.type);
      if (matchedCategory) setCategoryId(matchedCategory.id);
      const matchedAccount = accounts.find((a) => a.name === result.account);
      if (matchedAccount) setAccountId(matchedAccount.id);

      setNotice('AI 已解析，请确认后点击“保存账单”');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'AI 解析失败');
    }
  }

  return (
    <PageFrame title="账单明细" subtitle="支持手动记账和 AI 对话记账">
      {error ? <div className="error">{error}</div> : null}
      {notice ? <div className="success">{notice}</div> : null}

      <div className="grid two">
        <form className="card grid" onSubmit={submit}>
          <h3 style={{ margin: 0 }}>记一笔</h3>
          <select value={type} onChange={(e) => setType(e.target.value as 'expense' | 'income')}>
            <option value="expense">支出</option>
            <option value="income">收入</option>
          </select>
          <input
            type="number"
            step="0.01"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="金额"
            required
          />
          <select value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))} required>
            {filteredCategories.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <select value={accountId} onChange={(e) => setAccountId(Number(e.target.value))} required>
            {accounts.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <input value={note} onChange={(e) => setNote(e.target.value)} placeholder="备注" required />
          <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="标签，用逗号分隔" />
          <input
            type="datetime-local"
            value={occurredAt}
            onChange={(e) => setOccurredAt(e.target.value)}
            required
          />
          <button className="primary" type="submit">
            保存账单
          </button>
        </form>

        <div className="card grid">
          <h3 style={{ margin: 0 }}>AI 对话记账</h3>
          <textarea
            rows={5}
            value={aiText}
            onChange={(e) => setAiText(e.target.value)}
            placeholder="例如：今天午饭35"
          />
          <button className="secondary" type="button" onClick={runAIParse}>
            AI 解析
          </button>
          <p className="muted">解析后会填充左侧表单，你确认后再保存，符合你的“先确认卡片再入账”要求。</p>
        </div>
      </div>

      <div className="card">
        <h3>账单列表</h3>
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>类型</th>
              <th>金额</th>
              <th>分类</th>
              <th>账户</th>
              <th>备注</th>
              <th>标签</th>
            </tr>
          </thead>
          <tbody>
            {transactions.map((row) => (
              <tr key={row.id}>
                <td>{new Date(row.occurredAt).toLocaleString()}</td>
                <td>{row.type === 'income' ? '收入' : '支出'}</td>
                <td>¥ {row.amount.toFixed(2)}</td>
                <td>{row.category?.name}</td>
                <td>{row.account?.name}</td>
                <td>{row.note}</td>
                <td>{row.tags}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
