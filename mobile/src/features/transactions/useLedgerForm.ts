import { useEffect, useMemo, useState } from 'react';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';
import { createTransaction, parseAIText } from './api';

export function useLedgerForm(accounts: Account[], categories: Category[], reload: () => Promise<unknown>) {
  const [txType, setTxType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);
  const [note, setNote] = useState('');
  const [aiText, setAiText] = useState('今天午饭35');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  const filteredCategories = useMemo(() => categories.filter((c) => c.type === txType), [categories, txType]);

  useEffect(() => {
    if (accounts[0] && accountId === 0) setAccountId(accounts[0].id);
  }, [accounts, accountId]);

  useEffect(() => {
    const selected = categories.find((item) => item.id === categoryId);
    if (selected?.type === txType) return;
    const first = filteredCategories[0];
    setCategoryId(first?.id || 0);
  }, [categories, categoryId, filteredCategories, txType]);

  async function save() {
    const nextAmount = Number(amount);
    if (!Number.isFinite(nextAmount) || nextAmount <= 0) {
      setError('请输入大于 0 的金额');
      return false;
    }
    if (!categoryId || !accountId) {
      setError('请选择分类和账户');
      return false;
    }
    if (!note.trim()) {
      setError('请输入备注');
      return false;
    }

    try {
      setError('');
      setMessage('');
      await createTransaction({
        type: txType,
        amount: nextAmount,
        categoryId,
        accountId,
        note: note.trim(),
        tags: [],
        source: 'manual',
        occurredAt: new Date().toISOString(),
      });
      setAmount('');
      setNote('');
      setMessage('账单已保存');
      await reload();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
      return false;
    }
  }

  async function parseAI() {
    try {
      setError('');
      const resp = await parseAIText(aiText);
      const result = resp.result;
      setTxType(result.type);
      setAmount(String(result.amount));
      setNote(result.note || aiText);

      const matchedCategory = categories.find((item) => item.name === result.category && item.type === result.type);
      if (matchedCategory) setCategoryId(matchedCategory.id);

      const matchedAccount = accounts.find((item) => item.name === result.account);
      if (matchedAccount) setAccountId(matchedAccount.id);

      setMessage('AI 已解析，请确认后保存');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'AI 解析失败');
    }
  }

  return {
    txType,
    amount,
    note,
    aiText,
    categoryId,
    accountId,
    filteredCategories,
    message,
    error,
    setError,
    setTxType,
    setAmount,
    setNote,
    setAiText,
    setCategoryId,
    setAccountId,
    save,
    parseAI,
  };
}
