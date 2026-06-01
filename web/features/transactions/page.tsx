'use client';

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { Account, Category, Transaction, TransactionType } from '@/lib/types';
import { AIParsePanel } from './components/AIParsePanel';
import { TransactionForm } from './components/TransactionForm';
import { TransactionList } from './components/TransactionList';
import { createTransaction, listAccounts, listCategories, listTransactions, parseAIText } from './api';

export default function TransactionsFeaturePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [parsing, setParsing] = useState(false);
  const amountInputRef = useRef<HTMLInputElement>(null);
  const initializedRef = useRef(false);

  const [type, setType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState<number>(0);
  const [accountId, setAccountId] = useState<number>(0);
  const [note, setNote] = useState('');
  const [tags, setTags] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => new Date().toISOString().slice(0, 16));
  const [aiText, setAiText] = useState('今天午饭35');

  const filteredCategories = useMemo(() => categories.filter((c) => c.type === type), [categories, type]);

  const loadData = useCallback(async () => {
    setError('');
    try {
      setLoading(true);
      const [a, c, t] = await Promise.all([listAccounts(), listCategories(), listTransactions()]);
      setAccounts(a);
      setCategories(c);
      setTransactions(t.items || []);

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
  }, [type]);

  async function reloadTransactions() {
    try {
      const t = await listTransactions();
      setTransactions(t.items || []);
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
      await createTransaction({
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
      });
      setNotice('账单已保存');
      setAmount('');
      setNote('');
      setTags('');
      await reloadTransactions();
      requestAnimationFrame(() => amountInputRef.current?.focus());
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

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
          disabled={loading || saving}
        />
        <AIParsePanel aiText={aiText} amount={amount} disabled={loading || parsing} onAITextChange={setAiText} onParse={runAIParse} />
      </div>

      <TransactionList transactions={transactions} />
    </PageFrame>
  );
}
