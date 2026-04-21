'use client';

import { useEffect, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';
import { Transaction } from '@/lib/types';

type Summary = {
  income: number;
  expense: number;
  balance: number;
};

export default function OverviewPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [items, setItems] = useState<Transaction[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        const [s, tx] = await Promise.all([
          request<Summary>('/reports/summary'),
          request<{ items: Transaction[] }>('/transactions?page=1&pageSize=5'),
        ]);
        setSummary(s);
        setItems(tx.items || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame title="总览" subtitle="查看当期收支和最近账单">
      {error ? <div className="error">{error}</div> : null}
      <div className="grid two">
        <div className="card">
          <div className="muted">总收入</div>
          <h3>¥ {summary?.income?.toFixed(2) || '0.00'}</h3>
        </div>
        <div className="card">
          <div className="muted">总支出</div>
          <h3>¥ {summary?.expense?.toFixed(2) || '0.00'}</h3>
        </div>
      </div>
      <div className="card">
        <div className="muted">结余</div>
        <h3>¥ {summary?.balance?.toFixed(2) || '0.00'}</h3>
      </div>
      <div className="card">
        <h3>最近账单</h3>
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>类型</th>
              <th>金额</th>
              <th>分类</th>
              <th>账户</th>
              <th>备注</th>
            </tr>
          </thead>
          <tbody>
            {items.map((row) => (
              <tr key={row.id}>
                <td>{new Date(row.occurredAt).toLocaleString()}</td>
                <td>{row.type === 'income' ? '收入' : '支出'}</td>
                <td>¥ {row.amount.toFixed(2)}</td>
                <td>{row.category?.name}</td>
                <td>{row.account?.name}</td>
                <td>{row.note}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
