'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { PageFrame } from '@/components/PageFrame';
import type { Summary, Transaction } from '@/lib/types';
import { getOverviewSummary, listRecentTransactions } from './api';
import { HeroSummary } from './components/HeroSummary';
import { QuickActions } from './components/QuickActions';
import { RecentTransactions } from './components/RecentTransactions';

export default function OverviewFeaturePage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [items, setItems] = useState<Transaction[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        const [s, tx] = await Promise.all([getOverviewSummary(), listRecentTransactions()]);
        setSummary(s);
        setItems(tx.items || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame
      title="今天也好好记账"
      subtitle="参考手机记账产品的轻量工作流，把结余、快速入口和最近账单放在第一屏。"
      action={
        <Link className="primary" href="/transactions">
          + 记一笔
        </Link>
      }
    >
      {error ? <div className="error">{error}</div> : null}
      <HeroSummary summary={summary} />
      <QuickActions />

      <div className="grid two">
        <section className="card stat-card">
          <span className="label">本月记账状态</span>
          <span className="value">{items.length}</span>
          <span className="hint">最近加载的账单条目，保持每日记录后趋势会更稳定。</span>
        </section>
        <section className="card stat-card">
          <span className="label">支出占收入</span>
          <span className="value">
            {summary?.income ? `${Math.round((summary.expense / summary.income) * 100)}%` : '0%'}
          </span>
          <span className="hint">当比例接近预算上限时，可以优先查看分类分析。</span>
        </section>
      </div>

      <RecentTransactions items={items} />
    </PageFrame>
  );
}
