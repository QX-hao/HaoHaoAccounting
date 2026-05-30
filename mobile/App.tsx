import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, SafeAreaView, ScrollView, Text } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { LoginScreen } from './src/features/auth/LoginScreen';
import { login, verifyCurrentUser } from './src/features/auth/api';
import { loadDashboardData } from './src/features/dashboard/api';
import { HomeScreen } from './src/features/home/HomeScreen';
import { createSimpleAccount, createSimpleCategory } from './src/features/profile/api';
import { ProfileScreen } from './src/features/profile/ProfileScreen';
import { ReportsScreen } from './src/features/reports/ReportsScreen';
import { AddTransactionScreen } from './src/features/transactions/AddTransactionScreen';
import { createTransaction, parseAIText } from './src/features/transactions/api';
import { TabBar } from './src/navigation/TabBar';
import { clearToken, getToken, setToken } from './src/shared/api/client';
import type { Account, Category, Summary, Tab, Transaction, TransactionType } from './src/shared/types/accounting';
import { styles } from './src/theme/styles';

export default function App() {
  const [ready, setReady] = useState(false);
  const [authed, setAuthed] = useState(false);
  const [tab, setTab] = useState<Tab>('home');

  const [summary, setSummary] = useState<Summary | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);

  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');

  const [txType, setTxType] = useState<TransactionType>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);
  const [note, setNote] = useState('');
  const [aiText, setAiText] = useState('今天午饭35');

  const filteredCategories = useMemo(() => categories.filter((c) => c.type === txType), [categories, txType]);

  async function loadAll() {
    try {
      setError('');
      const data = await loadDashboardData();
      setSummary(data.summary);
      setTransactions(data.transactions);
      setAccounts(data.accounts);
      setCategories(data.categories);
      if (data.accounts[0] && accountId === 0) setAccountId(data.accounts[0].id);
      if (data.categories[0] && categoryId === 0) setCategoryId(data.categories[0].id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  useEffect(() => {
    async function boot() {
      const token = await getToken();
      if (token) {
        try {
          await verifyCurrentUser();
          setAuthed(true);
          await loadAll();
        } catch {
          await clearToken();
          setAuthed(false);
        }
      }
      setReady(true);
    }
    boot();
  }, []);

  useEffect(() => {
    const first = filteredCategories[0];
    if (first) setCategoryId(first.id);
  }, [txType, categories]);

  async function handleLogin() {
    const nextUsername = username.trim();
    const nextPassword = password.trim();
    if (!nextUsername) {
      setError('请输入用户名');
      return;
    }
    if (!nextPassword) {
      setError('请输入密码');
      return;
    }

    try {
      setError('');
      const resp = await login({ username: nextUsername, password: nextPassword });
      await setToken(resp.token);
      setAuthed(true);
      await loadAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    }
  }

  async function handleSaveTransaction() {
    try {
      setError('');
      setMessage('');
      await createTransaction({
        type: txType,
        amount: Number(amount),
        categoryId,
        accountId,
        note,
        tags: [],
        source: 'manual',
        occurredAt: new Date().toISOString(),
      });
      setAmount('');
      setNote('');
      setMessage('账单已保存');
      await loadAll();
      setTab('home');
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    }
  }

  async function handleAIParse() {
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

  async function handleLogout() {
    await clearToken();
    setAuthed(false);
    setSummary(null);
    setTransactions([]);
  }

  async function handleCreateSimpleCategory() {
    await createSimpleCategory(`自定义${Date.now().toString().slice(-4)}`);
    await loadAll();
  }

  async function handleCreateSimpleAccount() {
    await createSimpleAccount(`账户${Date.now().toString().slice(-4)}`);
    await loadAll();
  }

  if (!ready) {
    return (
      <SafeAreaView style={styles.centered}>
        <ActivityIndicator />
      </SafeAreaView>
    );
  }

  if (!authed) {
    return (
      <SafeAreaView style={styles.container}>
        <LoginScreen
          username={username}
          password={password}
          error={error}
          onUsernameChange={setUsername}
          onPasswordChange={setPassword}
          onLogin={handleLogin}
        />
      </SafeAreaView>
    );
  }

  const categoryName = filteredCategories.find((v) => v.id === categoryId)?.name || '';
  const accountName = accounts.find((v) => v.id === accountId)?.name || '';

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar style="dark" />
      <ScrollView contentContainerStyle={styles.scroll}>
        <Text style={styles.title}>好好记账</Text>
        {!!error && <Text style={styles.error}>{error}</Text>}
        {!!message && <Text style={styles.success}>{message}</Text>}

        {tab === 'home' && <HomeScreen summary={summary} transactions={transactions} />}
        {tab === 'add' && (
          <AddTransactionScreen
            txType={txType}
            amount={amount}
            note={note}
            aiText={aiText}
            categoryName={categoryName}
            accountName={accountName}
            onTxTypeChange={setTxType}
            onAmountChange={setAmount}
            onNoteChange={setNote}
            onAITextChange={setAiText}
            onSave={handleSaveTransaction}
            onAIParse={handleAIParse}
          />
        )}
        {tab === 'reports' && <ReportsScreen summary={summary} />}
        {tab === 'profile' && (
          <ProfileScreen
            onCreateCategory={handleCreateSimpleCategory}
            onCreateAccount={handleCreateSimpleAccount}
            onLogout={handleLogout}
          />
        )}
      </ScrollView>

      <TabBar activeTab={tab} onTabChange={setTab} />
    </SafeAreaView>
  );
}
