'use client';

import { FormEvent, useEffect, useMemo, useState } from 'react';
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

  const [type, setType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState<number>(0);
  const [accountId, setAccountId] = useState<number>(0);
  const [note, setNote] = useState('');
  const [tags, setTags] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => new Date().toISOString().slice(0, 16));
  const [aiText, setAiText] = useState('今天午饭35');

  const filteredCategories = useMemo(() => categories.filter((c) => c.type === type), [categories, type]);

  async function loadData() {
    setError('');
    try {
      const [a, c, t] = await Promise.all([listAccounts(), listCategories(), listTransactions()]);
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
      await createTransaction({
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
    }
  }

  return (
    <PageFrame
      title="记一笔"
      subtitle="金额先行、分类轻点、AI 解析辅助确认，尽量接近手机记账 App 的操作节奏。"
    >
      {error ? <div className="error">{error}</div> : null}
      {notice ? <div className="success">{notice}</div> : null}

      <div className="grid two">
        <TransactionForm
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
        />
        <AIParsePanel aiText={aiText} amount={amount} onAITextChange={setAiText} onParse={runAIParse} />
      </div>

      <TransactionList transactions={transactions} />
    </PageFrame>
  );
}
