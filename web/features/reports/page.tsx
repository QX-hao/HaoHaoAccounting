'use client';

import { useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Summary } from '@/lib/types';
import { getReportSummary } from './api';
import { BarBreakdown } from './components/BarBreakdown';
import { MonthlyTrend } from './components/MonthlyTrend';
import { PeriodCompare } from './components/PeriodCompare';
import { SummaryCards } from './components/SummaryCards';

export default function ReportsFeaturePage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        setSummary(await getReportSummary());
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame title="统计" subtitle="把分类占比、账户支出和月度趋势压缩到更容易扫读的卡片里。">
      {error ? <div className="error">{error}</div> : null}
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
