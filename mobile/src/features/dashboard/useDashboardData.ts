import { useState } from 'react';
import type { Account, Category, Summary, Transaction } from '../../shared/types/accounting';
import { loadDashboardData } from './api';

export function useDashboardData() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [transactionTotal, setTransactionTotal] = useState(0);
  const [error, setError] = useState('');

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
