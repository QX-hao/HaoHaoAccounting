'use client';

import { useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import { listAccounts } from '@/features/accounts/api';
import { listCategories } from '@/features/categories/api';
import type { Account, Category, Summary } from '@/lib/types';
import { getReportSummary } from './api';
import { BarBreakdown } from './components/BarBreakdown';
import { MonthlyTrend } from './components/MonthlyTrend';
import { PeriodCompare } from './components/PeriodCompare';
import { SummaryCards } from './components/SummaryCards';

type Preset = 'current' | 'previous' | 'custom';

export default function ReportsFeaturePage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [preset, setPreset] = useState<Preset>('current');
  const [start, setStart] = useState(() => monthRange(new Date()).start);
  const [end, setEnd] = useState(() => monthRange(new Date()).end);
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [nextSummary, nextAccounts, nextCategories] = await Promise.all([
          getReportSummary({ start, end, categoryId, accountId }),
          listAccounts(),
          listCategories(),
        ]);
        setSummary(nextSummary);
        setAccounts(nextAccounts);
        setCategories(nextCategories);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [start, end, categoryId, accountId]);

  function selectPreset(nextPreset: Preset) {
    setPreset(nextPreset);
    if (nextPreset === 'custom') return;
    const base = new Date();
    if (nextPreset === 'previous') {
      base.setMonth(base.getMonth() - 1);
    }
    const range = monthRange(base);
    setStart(range.start);
    setEnd(range.end);
  }

  return (
    <PageFrame title="统计" subtitle="把分类占比、账户支出和月度趋势压缩到更容易扫读的卡片里。">
      {error ? <div className="error">{error}</div> : null}
      {loading ? <div className="notice">正在加载统计数据...</div> : null}

      <section className="panel grid">
        <div className="toolbar">
          {(['current', 'previous', 'custom'] as const).map((item) => (
            <button className={preset === item ? 'secondary' : 'ghost'} key={item} type="button" onClick={() => selectPreset(item)}>
              {presetLabel(item)}
            </button>
          ))}
        </div>
        <div className="form-grid">
          <input type="date" value={start} onChange={(e) => {
            setPreset('custom');
            setStart(e.target.value);
          }} />
          <input type="date" value={end} onChange={(e) => {
            setPreset('custom');
            setEnd(e.target.value);
          }} />
          <select value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}>
            <option value={0}>全部分类</option>
            {categories.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <select value={accountId} onChange={(e) => setAccountId(Number(e.target.value))}>
            <option value={0}>全部账户</option>
            {accounts.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </div>
      </section>

      <SummaryCards summary={summary} />

      <div className="grid two">
        <BarBreakdown
          eyebrow="Category"
          title="分类占比"
          badge="支出"
          empty="暂无分类数据。"
          expense
          items={(summary?.byCategory || []).map((item) => ({ label: item.category, amount: item.amount }))}
        />
        <BarBreakdown
          eyebrow="Account"
          title="账户支出"
          badge="账户"
          empty="暂无账户数据。"
          items={(summary?.byAccount || []).map((item) => ({ label: item.account, amount: item.amount }))}
        />
      </div>

      <MonthlyTrend items={summary?.monthlyTrend || []} />
      <PeriodCompare summary={summary} />
    </PageFrame>
  );
}

function monthRange(date: Date) {
  const start = new Date(date.getFullYear(), date.getMonth(), 1);
  const end = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  return {
    start: formatDateInput(start),
    end: formatDateInput(end),
  };
}

function formatDateInput(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function presetLabel(preset: Preset) {
  if (preset === 'current') return '本月';
  if (preset === 'previous') return '上月';
  return '自定义';
}
