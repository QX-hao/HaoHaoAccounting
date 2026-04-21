'use client';

import { useEffect, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';

type CategoryStat = { category: string; amount: number };
type AccountStat = { account: string; amount: number };
type MonthTrend = { month: string; income: number; expense: number };

type Summary = {
  income: number;
  expense: number;
  balance: number;
  byCategory: CategoryStat[];
  byAccount: AccountStat[];
  monthlyTrend: MonthTrend[];
  periodCompare: {
    current: { income: number; expense: number };
    previous: { income: number; expense: number };
  };
};

export default function ReportsPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        const resp = await request<Summary>('/reports/summary');
        setSummary(resp);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame title="报表分析" subtitle="分类占比、月度趋势、按账户统计、时间段对比">
      {error ? <div className="error">{error}</div> : null}
      <div className="grid two">
        <div className="card">
          <div className="muted">本期收入</div>
          <h3>¥ {summary?.income?.toFixed(2) || '0.00'}</h3>
        </div>
        <div className="card">
          <div className="muted">本期支出</div>
          <h3>¥ {summary?.expense?.toFixed(2) || '0.00'}</h3>
        </div>
      </div>

      <div className="card">
        <h3>分类占比（支出）</h3>
        <table>
          <thead>
            <tr>
              <th>分类</th>
              <th>金额</th>
            </tr>
          </thead>
          <tbody>
            {summary?.byCategory?.map((item) => (
              <tr key={item.category}>
                <td>{item.category}</td>
                <td>¥ {item.amount.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>按账户统计（支出）</h3>
        <table>
          <thead>
            <tr>
              <th>账户</th>
              <th>金额</th>
            </tr>
          </thead>
          <tbody>
            {summary?.byAccount?.map((item) => (
              <tr key={item.account}>
                <td>{item.account}</td>
                <td>¥ {item.amount.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>月度收支趋势</h3>
        <table>
          <thead>
            <tr>
              <th>月份</th>
              <th>收入</th>
              <th>支出</th>
            </tr>
          </thead>
          <tbody>
            {summary?.monthlyTrend?.map((item) => (
              <tr key={item.month}>
                <td>{item.month}</td>
                <td>¥ {item.income.toFixed(2)}</td>
                <td>¥ {item.expense.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>时间段对比</h3>
        <p>
          当前周期：收入 ¥ {summary?.periodCompare?.current?.income?.toFixed(2) || '0.00'} / 支出 ¥{' '}
          {summary?.periodCompare?.current?.expense?.toFixed(2) || '0.00'}
        </p>
        <p>
          上一周期：收入 ¥ {summary?.periodCompare?.previous?.income?.toFixed(2) || '0.00'} / 支出 ¥{' '}
          {summary?.periodCompare?.previous?.expense?.toFixed(2) || '0.00'}
        </p>
      </div>
    </PageFrame>
  );
}
