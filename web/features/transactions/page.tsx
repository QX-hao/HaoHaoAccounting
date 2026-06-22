'use client';

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Account, Category, Transaction, TransactionType } from '@/lib/types';
import { AIParsePanel } from './components/AIParsePanel';
import { TransactionForm } from './components/TransactionForm';
import { TransactionList } from './components/TransactionList';
import { createTransaction, deleteTransaction, listAccounts, listCategories, listTransactions, parseAIText, updateTransaction, type TransactionFilters } from './api';

const defaultPageSize = 20;

// 交易页负责把账户、分类、账单列表、筛选条件和编辑表单放在同一个状态闭环里。
export default function TransactionsFeaturePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [parsing, setParsing] = useState(false);
  const [editing, setEditing] = useState<Transaction | null>(null);
  const [total, setTotal] = useState(0);
  const amountInputRef = useRef<HTMLInputElement>(null);
  const initializedRef = useRef(false);

  const [filters, setFilters] = useState<TransactionFilters>({ page: 1, pageSize: defaultPageSize, type: '' });
  const [draftFilters, setDraftFilters] = useState({ start: '', end: '', type: '', categoryId: 0, accountId: 0, q: '' });

  const [type, setType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState<number>(0);
  const [accountId, setAccountId] = useState<number>(0);
  const [note, setNote] = useState('');
  const [tags, setTags] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => new Date().toISOString().slice(0, 16));
  const [aiText, setAiText] = useState('今天午饭35');

  // 分类选项跟随当前收支类型变化，避免把收入账单保存到支出分类下。
  const filteredCategories = useMemo(() => categories.filter((c) => c.type === type), [categories, type]);

  // 首次加载会同时取账户、分类和账单，并为表单填入默认账户/分类。
  const loadData = useCallback(async () => {
    setError('');
    try {
      setLoading(true);
      const [a, c, t] = await Promise.all([listAccounts(), listCategories(), listTransactions(filters)]);
      setAccounts(a);
      setCategories(c);
      setTransactions(t.items || []);
      setTotal(t.pagination?.total ?? t.items?.length ?? 0);

      if (!initializedRef.current) {
        if (a[0]) setAccountId(a[0].id);
        const firstCategory = c.find((item) => item.type === type);
        if (firstCategory) setCategoryId(firstCategory.id);
        initializedRef.current = true;
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [filters, type]);

  // 保存、删除后只刷新账单列表，避免不必要地重置账户/分类和当前表单状态。
  async function reloadTransactions() {
    try {
      const t = await listTransactions(filters);
      setTransactions(t.items || []);
      setTotal(t.pagination?.total ?? t.items?.length ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    if (!initializedRef.current) return;
    const selected = categories.find((item) => item.id === categoryId);
    if (selected?.type === type) return;
    const firstCategory = categories.find((item) => item.type === type);
    if (firstCategory) {
      setCategoryId(firstCategory.id);
    } else {
      setCategoryId(0);
    }
  }, [categories, categoryId, type]);

  // submit 同时处理新增和编辑，后端会负责交易写入和账户余额联动。
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (saving) return;
    setError('');
    setNotice('');

    const nextAmount = Number(amount);
    if (!Number.isFinite(nextAmount) || nextAmount <= 0) {
      setError('请输入大于 0 的金额');
      amountInputRef.current?.focus();
      return;
    }
    if (!accountId) {
      setError('请先选择或创建账户');
      return;
    }
    if (!categoryId) {
      setError('请先选择或创建分类');
      return;
    }
    if (!note.trim()) {
      setError('请输入备注');
      return;
    }

    try {
      setSaving(true);
      const payload = {
        type,
        amount: nextAmount,
        categoryId,
        accountId,
        note: note.trim(),
        tags: tags
          .split(',')
          .map((v) => v.trim())
          .filter(Boolean),
        occurredAt: new Date(occurredAt).toISOString(),
        source: 'manual',
      };
      if (editing) {
        await updateTransaction(editing.id, payload);
        setNotice('账单已更新');
      } else {
        await createTransaction(payload);
        setNotice('账单已保存');
      }
      resetForm();
      await reloadTransactions();
      requestAnimationFrame(() => amountInputRef.current?.focus());
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  // 筛选条件先存在草稿里，点击查询时再真正刷新列表，避免输入过程中频繁请求接口。
  function applyFilters(e: FormEvent) {
    e.preventDefault();
    setFilters({
      page: 1,
      pageSize: filters.pageSize,
      start: draftFilters.start,
      end: draftFilters.end,
      type: draftFilters.type as '' | TransactionType,
      categoryId: draftFilters.categoryId,
      accountId: draftFilters.accountId,
      q: draftFilters.q,
    });
  }

  function resetFilters() {
    setDraftFilters({ start: '', end: '', type: '', categoryId: 0, accountId: 0, q: '' });
    setFilters({ page: 1, pageSize: defaultPageSize, type: '' });
  }

  // 分页只改变 filters.page，列表刷新由 loadData 的依赖变化统一触发。
  function changePage(page: number) {
    setFilters((current) => ({ ...current, page }));
  }

  // 编辑时把列表行回填到表单，保留原始分类/账户 ID，避免只依赖预加载对象。
  function startEdit(row: Transaction) {
    setEditing(row);
    setType(row.type);
    setAmount(String(row.amount));
    setCategoryId(row.categoryId || row.category?.id || 0);
    setAccountId(row.accountId || row.account?.id || 0);
    setNote(row.note || '');
    setTags(row.tags || '');
    setOccurredAt(toDateTimeLocal(row.occurredAt));
    setError('');
    setNotice('');
    requestAnimationFrame(() => amountInputRef.current?.focus());
  }

  async function remove(row: Transaction) {
    const ok = window.confirm(`确定删除账单「${row.note || row.category?.name || row.id}」吗？删除后会同步回滚账户余额。`);
    if (!ok) return;
    setError('');
    setNotice('');
    try {
      setSaving(true);
      await deleteTransaction(row.id);
      if (editing?.id === row.id) resetForm();
      setNotice('账单已删除');
      await reloadTransactions();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    } finally {
      setSaving(false);
    }
  }

  function resetForm() {
    setEditing(null);
    setAmount('');
    setNote('');
    setTags('');
    setOccurredAt(new Date().toISOString().slice(0, 16));
  }

  // AI 解析只负责预填表单，仍需用户确认后才会真正保存账单。
  async function runAIParse() {
    if (parsing) return;
    setError('');
    setNotice('');
    if (!aiText.trim()) {
      setError('请输入要解析的记账文本');
      return;
    }
    try {
      setParsing(true);
      const resp = await parseAIText(aiText);
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
    } finally {
      setParsing(false);
    }
  }

  return (
    <PageFrame
      title="记一笔"
      subtitle="金额先行、分类轻点、AI 解析辅助确认，尽量接近手机记账 App 的操作节奏。"
    >
      {error ? <div className="error">{error}</div> : null}
      {notice ? <div className="success">{notice}</div> : null}
      {loading ? <div className="notice">正在加载账户、分类和最近账单...</div> : null}

      <div className="grid two">
        <TransactionForm
          ref={amountInputRef}
          accounts={accounts}
          filteredCategories={filteredCategories}
          type={type}
          amount={amount}
          categoryId={categoryId}
          accountId={accountId}
          note={note}
          tags={tags}
          occurredAt={occurredAt}
          onTypeChange={setType}
          onAmountChange={setAmount}
          onCategoryChange={setCategoryId}
          onAccountChange={setAccountId}
          onNoteChange={setNote}
          onTagsChange={setTags}
          onOccurredAtChange={setOccurredAt}
          onSubmit={submit}
          onCancelEdit={resetForm}
          disabled={loading || saving}
          editing={Boolean(editing)}
        />
        <AIParsePanel aiText={aiText} amount={amount} disabled={loading || parsing} onAITextChange={setAiText} onParse={runAIParse} />
      </div>

      <form className="panel grid" onSubmit={applyFilters}>
        <div className="hero-topline">
          <div>
            <span className="eyebrow">Filter</span>
            <h3>筛选账单</h3>
          </div>
          <button className="ghost" type="button" onClick={resetFilters}>
            重置
          </button>
        </div>
        <div className="form-grid">
          <input type="date" value={draftFilters.start} onChange={(e) => setDraftFilters((v) => ({ ...v, start: e.target.value }))} />
          <input type="date" value={draftFilters.end} onChange={(e) => setDraftFilters((v) => ({ ...v, end: e.target.value }))} />
          <select value={draftFilters.type} onChange={(e) => setDraftFilters((v) => ({ ...v, type: e.target.value }))}>
            <option value="">全部类型</option>
            <option value="expense">支出</option>
            <option value="income">收入</option>
          </select>
          <select value={draftFilters.categoryId} onChange={(e) => setDraftFilters((v) => ({ ...v, categoryId: Number(e.target.value) }))}>
            <option value={0}>全部分类</option>
            {categories.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <select value={draftFilters.accountId} onChange={(e) => setDraftFilters((v) => ({ ...v, accountId: Number(e.target.value) }))}>
            <option value={0}>全部账户</option>
            {accounts.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <input value={draftFilters.q} onChange={(e) => setDraftFilters((v) => ({ ...v, q: e.target.value }))} placeholder="搜索备注或标签" />
        </div>
        <div className="toolbar">
          <button className="secondary" type="submit" disabled={loading}>
            应用筛选
          </button>
          <select value={filters.pageSize} onChange={(e) => setFilters((current) => ({ ...current, page: 1, pageSize: Number(e.target.value) }))}>
            <option value={10}>每页 10 条</option>
            <option value={20}>每页 20 条</option>
            <option value={50}>每页 50 条</option>
          </select>
        </div>
      </form>

      <TransactionList
        transactions={transactions}
        total={total}
        page={filters.page}
        pageSize={filters.pageSize}
        disabled={saving || loading}
        onEdit={startEdit}
        onDelete={remove}
        onPageChange={changePage}
      />
    </PageFrame>
  );
}

function toDateTimeLocal(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return new Date().toISOString().slice(0, 16);
  }
  const offsetMs = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16);
}
