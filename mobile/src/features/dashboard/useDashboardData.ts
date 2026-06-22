import { useState } from 'react';
import type { Account, Category, Summary, Transaction } from '../../shared/types/accounting';
import { loadDashboardData } from './api';

// useDashboardData 聚合首页需要的报表、最近账单、账户和分类，页面只消费整理后的状态。
export function useDashboardData() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [transactionTotal, setTransactionTotal] = useState(0);
  const [error, setError] = useState('');

  // loadAll 一次性拉取首页数据，返回原始数据方便调用方在登录后继续做初始化判断。
  async function loadAll() {
    try {
      setError('');
      const data = await loadDashboardData();
      setSummary(data.summary);
      setTransactions(data.transactions);
      setTransactionTotal(data.transactionTotal || data.transactions.length);
      setAccounts(data.accounts);
      setCategories(data.categories);
      return data;
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
      return null;
    }
  }

  // clear 用于登出或 token 失效时清空敏感账本数据。
  function clear() {
    setSummary(null);
    setTransactions([]);
    setTransactionTotal(0);
    setAccounts([]);
    setCategories([]);
  }

  return {
    summary,
    transactions,
    transactionTotal,
    accounts,
    categories,
    error,
    setError,
    loadAll,
    clear,
  };
}
