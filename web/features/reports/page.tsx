'use client';

import { useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import { listAccounts } from '@/features/accounts/api';
import { createBudget, deleteBudget, listBudgets, updateBudget } from '@/features/budgets/api';
import { listCategories } from '@/features/categories/api';
import type { Account, Budget, Category, Summary } from '@/lib/types';
import { getReportSummary } from './api';
import { AccountBalanceTrendPanel } from './components/AccountBalanceTrendPanel';
import { BarBreakdown } from './components/BarBreakdown';
import { BudgetExecutionPanel } from './components/BudgetExecutionPanel';
import { BudgetManager } from './components/BudgetManager';
import { CategoryTrendPanel } from './components/CategoryTrendPanel';
import { PeriodCompare } from './components/PeriodCompare';
import { SummaryCards } from './components/SummaryCards';
import { SummaryTables } from './components/SummaryTables';
import { TrendChart } from './components/TrendChart';

type Preset = 'current' | 'previous' | 'custom';
type TrendGranularity = 'day' | 'week' | 'month';

// 统计页集中管理报表筛选和预算编辑状态，图表组件只负责渲染传入的数据。
export default function ReportsFeaturePage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [budgets, setBudgets] = useState<Budget[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [preset, setPreset] = useState<Preset>('current');
  const [start, setStart] = useState(() => monthRange(new Date()).start);
  const [end, setEnd] = useState(() => monthRange(new Date()).end);
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);
  const [trend, setTrend] = useState<TrendGranularity>('month');
  const [budgetMonth, setBudgetMonth] = useState(() => monthValue(new Date()));
  const [budgetCategoryId, setBudgetCategoryId] = useState(0);
  const [budgetAmount, setBudgetAmount] = useState('');
  const [editingBudgetId, setEditingBudgetId] = useState(0);

  useEffect(() => {
    // 报表、账户、分类、预算一起加载，保证筛选下拉和图表数据保持同一时间点的状态。
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [nextSummary, nextAccounts, nextCategories, nextBudgets] = await Promise.all([
          getReportSummary({ start, end, categoryId, accountId, trend }),
          listAccounts(),
          listCategories(),
          listBudgets(budgetMonth),
        ]);
        setSummary(nextSummary);
        setAccounts(nextAccounts);
        setCategories(nextCategories);
        setBudgets(nextBudgets);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [start, end, categoryId, accountId, trend, budgetMonth]);

  // 预算变更后只刷新报表和预算列表，不重新拉账户/分类这些低频数据。
  async function refreshReports() {
    const [nextSummary, nextBudgets] = await Promise.all([
      getReportSummary({ start, end, categoryId, accountId, trend }),
      listBudgets(budgetMonth),
    ]);
    setSummary(nextSummary);
    setBudgets(nextBudgets);
  }

  // 预设范围只修改起止日期；选择自定义时保留用户当前输入。
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

  // 预算的 categoryId=0 表示全月总预算，后端报表会按同一个约定计算执行率。
  async function saveBudget() {
    try {
      setError('');
      const payload = {
        month: budgetMonth,
        categoryId: budgetCategoryId,
        amount: Number(budgetAmount || 0),
      };
      if (editingBudgetId) {
        await updateBudget(editingBudgetId, payload);
      } else {
        await createBudget(payload);
      }
      resetBudgetForm();
      await refreshReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : '预算保存失败');
    }
  }

  function editBudget(budget: Budget) {
    setEditingBudgetId(budget.id);
    setBudgetMonth(budget.month);
    setBudgetCategoryId(budget.categoryId);
    setBudgetAmount(String(budget.amount));
  }

  async function removeBudget(id: number) {
    try {
      setError('');
      await deleteBudget(id);
      if (editingBudgetId === id) resetBudgetForm();
      await refreshReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : '预算删除失败');
    }
  }

  function resetBudgetForm() {
    setEditingBudgetId(0);
    setBudgetCategoryId(0);
    setBudgetAmount('');
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
          <select value={trend} onChange={(e) => setTrend(e.target.value as TrendGranularity)}>
            <option value="day">按天趋势</option>
            <option value="week">按周趋势</option>
            <option value="month">按月趋势</option>
          </select>
        </div>
      </section>

      <SummaryCards summary={summary} />
      <BudgetManager
        budgets={budgets}
        categories={categories}
        month={budgetMonth}
        categoryId={budgetCategoryId}
        amount={budgetAmount}
        editingId={editingBudgetId}
        onMonthChange={setBudgetMonth}
        onCategoryChange={setBudgetCategoryId}
        onAmountChange={setBudgetAmount}
        onSave={saveBudget}
        onEdit={editBudget}
        onDelete={removeBudget}
        onCancelEdit={resetBudgetForm}
      />

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

      <TrendChart items={summary?.trend || summary?.monthlyTrend?.map((item) => ({ period: item.month, income: item.income, expense: item.expense })) || []} granularity={summary?.trendGranularity || trend} />
      <div className="grid two">
        <BudgetExecutionPanel items={summary?.budgetExecution || []} />
        <CategoryTrendPanel items={summary?.categoryTrend || []} />
      </div>
      <AccountBalanceTrendPanel items={summary?.accountBalanceTrend || []} />
      <SummaryTables daily={summary?.dailySummaries || []} monthly={summary?.monthlySummaries || []} />
      <PeriodCompare summary={summary} />
    </PageFrame>
  );
}

// monthRange 返回适配 <input type="date"> 的本月起止日期字符串。
function monthRange(date: Date) {
  const start = new Date(date.getFullYear(), date.getMonth(), 1);
  const end = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  return {
    start: formatDateInput(start),
    end: formatDateInput(end),
  };
}

// formatDateInput 避免 toISOString 按 UTC 转换导致本地日期前后偏一天。
function formatDateInput(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

// monthValue 生成预算接口使用的 yyyy-MM 月份键。
function monthValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  return `${year}-${month}`;
}

function presetLabel(preset: Preset) {
  if (preset === 'current') return '本月';
  if (preset === 'previous') return '上月';
  return '自定义';
}
