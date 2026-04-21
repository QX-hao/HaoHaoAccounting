import { useEffect, useMemo, useState } from 'react';
import {
  ActivityIndicator,
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { clearToken, getToken, request, setToken } from './src/api';

type Tab = 'home' | 'add' | 'reports' | 'profile';

type Account = { id: number; name: string; balance: number; type: string };
type Category = { id: number; name: string; type: 'income' | 'expense'; isSystem: boolean };
type Transaction = {
  id: number;
  type: 'income' | 'expense';
  amount: number;
  note: string;
  occurredAt: string;
  category: { name: string };
  account: { name: string };
};

type Summary = {
  income: number;
  expense: number;
  balance: number;
  byCategory: { category: string; amount: number }[];
  byAccount: { account: string; amount: number }[];
};

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

  const [loginType, setLoginType] = useState<'phone' | 'email' | 'wechat'>('phone');
  const [identifier, setIdentifier] = useState('');

  const [txType, setTxType] = useState<'expense' | 'income'>('expense');
  const [amount, setAmount] = useState('');
  const [categoryId, setCategoryId] = useState(0);
  const [accountId, setAccountId] = useState(0);
  const [note, setNote] = useState('');
  const [aiText, setAiText] = useState('今天午饭35');

  const filteredCategories = useMemo(
    () => categories.filter((c) => c.type === txType),
    [categories, txType],
  );

  useEffect(() => {
    async function boot() {
      const token = await getToken();
      setAuthed(Boolean(token));
      setReady(true);
      if (token) {
        await loadAll();
      }
    }
    boot();
  }, []);

  useEffect(() => {
    const first = filteredCategories[0];
    if (first) setCategoryId(first.id);
  }, [txType, categories]);

  async function loadAll() {
    try {
      setError('');
      const [s, tx, a, c] = await Promise.all([
        request<Summary>('/reports/summary'),
        request<{ items: Transaction[] }>('/transactions?page=1&pageSize=20'),
        request<Account[]>('/accounts'),
        request<Category[]>('/categories'),
      ]);
      setSummary(s);
      setTransactions(tx.items || []);
      setAccounts(a);
      setCategories(c);
      if (a[0] && accountId === 0) setAccountId(a[0].id);
      if (c[0] && categoryId === 0) setCategoryId(c[0].id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    }
  }

  async function handleLogin() {
    try {
      setError('');
      const resp = await request<{ token: string }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ loginType, identifier }),
      });
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
      await request('/transactions', {
        method: 'POST',
        body: JSON.stringify({
          type: txType,
          amount: Number(amount),
          categoryId,
          accountId,
          note,
          tags: [],
          source: 'manual',
          occurredAt: new Date().toISOString(),
        }),
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
      const resp = await request<{
        result: {
          type: 'income' | 'expense';
          amount: number;
          category: string;
          account: string;
          note: string;
        };
      }>('/ai/parse', {
        method: 'POST',
        body: JSON.stringify({ text: aiText }),
      });

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

  async function createSimpleCategory() {
    const name = `自定义${Date.now().toString().slice(-4)}`;
    await request('/categories', {
      method: 'POST',
      body: JSON.stringify({ name, type: 'expense' }),
    });
    await loadAll();
  }

  async function createSimpleAccount() {
    const name = `账户${Date.now().toString().slice(-4)}`;
    await request('/accounts', {
      method: 'POST',
      body: JSON.stringify({ name, type: 'custom', balance: 0 }),
    });
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
        <StatusBar style="dark" />
        <View style={styles.loginCard}>
          <Text style={styles.title}>登录好好记账</Text>
          <Text style={styles.muted}>手机号 / 邮箱 / 微信免密登录（MVP）</Text>

          <View style={styles.row}>
            {(['phone', 'email', 'wechat'] as const).map((item) => (
              <TouchableOpacity
                key={item}
                style={[styles.chip, loginType === item && styles.chipActive]}
                onPress={() => setLoginType(item)}
              >
                <Text style={loginType === item ? styles.chipTextActive : styles.chipText}>{item}</Text>
              </TouchableOpacity>
            ))}
          </View>

          <TextInput
            style={styles.input}
            placeholder="请输入账号"
            value={identifier}
            onChangeText={setIdentifier}
          />
          {!!error && <Text style={styles.error}>{error}</Text>}
          <TouchableOpacity style={styles.primaryBtn} onPress={handleLogin}>
            <Text style={styles.primaryBtnText}>登录</Text>
          </TouchableOpacity>
        </View>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar style="dark" />
      <ScrollView contentContainerStyle={styles.scroll}>
        <Text style={styles.title}>好好记账</Text>
        {!!error && <Text style={styles.error}>{error}</Text>}
        {!!message && <Text style={styles.success}>{message}</Text>}

        {tab === 'home' && (
          <View style={styles.card}>
            <Text style={styles.sectionTitle}>本月概览</Text>
            <Text>收入：¥ {summary?.income?.toFixed(2) || '0.00'}</Text>
            <Text>支出：¥ {summary?.expense?.toFixed(2) || '0.00'}</Text>
            <Text>结余：¥ {summary?.balance?.toFixed(2) || '0.00'}</Text>
            <Text style={[styles.sectionTitle, { marginTop: 12 }]}>最近账单</Text>
            {transactions.map((item) => (
              <View key={item.id} style={styles.listItem}>
                <Text>{item.note}</Text>
                <Text>
                  {item.type === 'income' ? '收入' : '支出'} ¥{item.amount.toFixed(2)} · {item.category?.name}
                </Text>
              </View>
            ))}
          </View>
        )}

        {tab === 'add' && (
          <View style={styles.card}>
            <Text style={styles.sectionTitle}>记一笔</Text>
            <View style={styles.row}>
              {(['expense', 'income'] as const).map((item) => (
                <TouchableOpacity
                  key={item}
                  style={[styles.chip, txType === item && styles.chipActive]}
                  onPress={() => setTxType(item)}
                >
                  <Text style={txType === item ? styles.chipTextActive : styles.chipText}>
                    {item === 'expense' ? '支出' : '收入'}
                  </Text>
                </TouchableOpacity>
              ))}
            </View>
            <TextInput
              style={styles.input}
              keyboardType="numeric"
              placeholder="金额"
              value={amount}
              onChangeText={setAmount}
            />
            <Text style={styles.muted}>分类：{filteredCategories.find((v) => v.id === categoryId)?.name || '-'}</Text>
            <Text style={styles.muted}>账户：{accounts.find((v) => v.id === accountId)?.name || '-'}</Text>
            <TextInput style={styles.input} placeholder="备注" value={note} onChangeText={setNote} />
            <TouchableOpacity style={styles.primaryBtn} onPress={handleSaveTransaction}>
              <Text style={styles.primaryBtnText}>保存账单</Text>
            </TouchableOpacity>

            <Text style={styles.sectionTitle}>AI 对话记账</Text>
            <TextInput
              style={[styles.input, { height: 80 }]}
              multiline
              value={aiText}
              onChangeText={setAiText}
            />
            <TouchableOpacity style={styles.secondaryBtn} onPress={handleAIParse}>
              <Text style={styles.secondaryBtnText}>AI 解析</Text>
            </TouchableOpacity>
          </View>
        )}

        {tab === 'reports' && (
          <View style={styles.card}>
            <Text style={styles.sectionTitle}>报表</Text>
            <Text style={styles.muted}>分类占比（支出）</Text>
            {summary?.byCategory?.map((item) => (
              <Text key={item.category}>
                {item.category}: ¥{item.amount.toFixed(2)}
              </Text>
            ))}
            <Text style={[styles.muted, { marginTop: 12 }]}>按账户统计（支出）</Text>
            {summary?.byAccount?.map((item) => (
              <Text key={item.account}>
                {item.account}: ¥{item.amount.toFixed(2)}
              </Text>
            ))}
          </View>
        )}

        {tab === 'profile' && (
          <View style={styles.card}>
            <Text style={styles.sectionTitle}>我的</Text>
            <TouchableOpacity style={styles.secondaryBtn} onPress={createSimpleCategory}>
              <Text style={styles.secondaryBtnText}>新增自定义分类</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.secondaryBtn} onPress={createSimpleAccount}>
              <Text style={styles.secondaryBtnText}>新增自定义账户</Text>
            </TouchableOpacity>
            <TouchableOpacity style={[styles.secondaryBtn, { backgroundColor: '#fee2e2' }]} onPress={handleLogout}>
              <Text style={{ color: '#991b1b', fontWeight: '600' }}>退出登录</Text>
            </TouchableOpacity>
          </View>
        )}
      </ScrollView>

      <View style={styles.tabBar}>
        {([
          ['home', '首页'],
          ['add', '记一笔'],
          ['reports', '报表'],
          ['profile', '我的'],
        ] as [Tab, string][]).map(([key, label]) => (
          <TouchableOpacity key={key} style={styles.tabBtn} onPress={() => setTab(key)}>
            <Text style={tab === key ? styles.tabActive : styles.tabText}>{label}</Text>
          </TouchableOpacity>
        ))}
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  loginCard: {
    margin: 20,
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    borderWidth: 1,
    borderColor: '#e2e8f0',
    gap: 10,
  },
  scroll: {
    padding: 16,
    paddingBottom: 100,
    gap: 12,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    color: '#1d4ed8',
  },
  card: {
    backgroundColor: '#fff',
    borderRadius: 12,
    borderWidth: 1,
    borderColor: '#e2e8f0',
    padding: 14,
    gap: 8,
  },
  sectionTitle: {
    marginTop: 4,
    fontSize: 16,
    fontWeight: '700',
  },
  muted: {
    color: '#64748b',
  },
  error: {
    color: '#b91c1c',
  },
  success: {
    color: '#166534',
  },
  row: {
    flexDirection: 'row',
    gap: 8,
    flexWrap: 'wrap',
  },
  chip: {
    borderRadius: 999,
    paddingVertical: 6,
    paddingHorizontal: 12,
    backgroundColor: '#e2e8f0',
  },
  chipActive: {
    backgroundColor: '#2563eb',
  },
  chipText: {
    color: '#334155',
    fontSize: 12,
  },
  chipTextActive: {
    color: '#fff',
    fontSize: 12,
  },
  input: {
    borderWidth: 1,
    borderColor: '#cbd5e1',
    borderRadius: 10,
    padding: 10,
    backgroundColor: '#fff',
  },
  primaryBtn: {
    borderRadius: 10,
    backgroundColor: '#2563eb',
    paddingVertical: 12,
    alignItems: 'center',
  },
  primaryBtnText: {
    color: '#fff',
    fontWeight: '700',
  },
  secondaryBtn: {
    borderRadius: 10,
    backgroundColor: '#eef2ff',
    paddingVertical: 12,
    alignItems: 'center',
  },
  secondaryBtnText: {
    color: '#3730a3',
    fontWeight: '600',
  },
  listItem: {
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#e2e8f0',
    gap: 2,
  },
  tabBar: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    flexDirection: 'row',
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    backgroundColor: '#fff',
    paddingVertical: 8,
  },
  tabBtn: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: 6,
  },
  tabText: {
    color: '#64748b',
  },
  tabActive: {
    color: '#1d4ed8',
    fontWeight: '700',
  },
});
