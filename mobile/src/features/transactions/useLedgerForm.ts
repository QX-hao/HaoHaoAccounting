import { useEffect, useMemo, useState } from 'react';
import type { Account, Category, Transaction, TransactionType } from '../../shared/types/accounting';
import { createTransaction, parseAIText, updateTransaction } from './api';

// useLedgerForm 负责移动端记账表单的完整状态：新增、编辑、AI 预填和保存后的刷新。
export function useLedgerForm(accounts: Account[], categories: Category[], reload: () => Promise<unknown>) {
  const [txType, setTxType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);
  const [note, setNote] = useState('');
  const [occurredAt, setOccurredAt] = useState('');
  const [aiText, setAiText] = useState('今天午饭35');
  const [editing, setEditing] = useState<Transaction | null>(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  // 分类跟随收支类型过滤，避免用户把一笔支出提交到收入分类。
  const filteredCategories = useMemo(() => categories.filter((c) => c.type === txType), [categories, txType]);

  useEffect(() => {
    // 账户列表加载完成后自动选中第一个账户，减少移动端记账前的必填操作。
    if (accounts[0] && accountId === 0) setAccountId(accounts[0].id);
  }, [accounts, accountId]);

  useEffect(() => {
    // 切换收支类型时，如果当前分类不匹配，就自动切到该类型下的第一个分类。
    const selected = categories.find((item) => item.id === categoryId);
    if (selected?.type === txType) return;
    const first = filteredCategories[0];
    setCategoryId(first?.id || 0);
  }, [categories, categoryId, filteredCategories, txType]);

  // save 同时处理新增和编辑；后端会在同一事务里同步维护账户余额。
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
      const payload = {
        type: txType,
        amount: nextAmount,
        categoryId,
        accountId,
        note: note.trim(),
        tags: [],
        source: 'manual',
        occurredAt: occurredAt.trim() || new Date().toISOString(),
      };
      if (editing) {
        await updateTransaction(editing.id, payload);
        setMessage('账单已更新');
      } else {
        await createTransaction(payload);
        setMessage('账单已保存');
      }
      resetForm();
      await reload();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
      return false;
    }
  }

  // startEdit 把列表里的账单回填到表单，允许用户在同一个表单里完成修改。
  function startEdit(transaction: Transaction) {
    setEditing(transaction);
    setTxType(transaction.type);
    setAmount(String(transaction.amount));
    setCategoryId(transaction.categoryId || transaction.category?.id || 0);
    setAccountId(transaction.accountId || transaction.account?.id || 0);
    setNote(transaction.note || '');
    setOccurredAt(transaction.occurredAt || '');
    setMessage('');
    setError('');
  }

  function resetForm() {
    setEditing(null);
    setAmount('');
    setNote('');
    setOccurredAt('');
  }

  // parseAI 只做表单预填，不直接保存，避免模型解析错误导致账单误写入。
  async function parseAI() {
    try {
      setError('');
      const resp = await parseAIText(aiText);
      const result = resp.result;
      setTxType(result.type);
      setAmount(String(result.amount));
      setNote(result.note || aiText);
      setOccurredAt(result.occurredAt || '');

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
    occurredAt,
    aiText,
    editing,
    categoryId,
    accountId,
    filteredCategories,
    message,
    error,
    setError,
    setTxType,
    setAmount,
    setNote,
    setOccurredAt,
    setAiText,
    setCategoryId,
    setAccountId,
    save,
    parseAI,
    startEdit,
    resetForm,
  };
}
