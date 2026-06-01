import { useEffect, useState } from 'react';
import { ActivityIndicator, SafeAreaView, ScrollView, Text } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { LoginScreen } from './src/features/auth/LoginScreen';
import { useSession } from './src/features/auth/useSession';
import { useDashboardData } from './src/features/dashboard/useDashboardData';
import { HomeScreen } from './src/features/home/HomeScreen';
import { createSimpleAccount, createSimpleCategory } from './src/features/profile/api';
import { ProfileScreen } from './src/features/profile/ProfileScreen';
import { ReportsScreen } from './src/features/reports/ReportsScreen';
import { AddTransactionScreen } from './src/features/transactions/AddTransactionScreen';
import { useLedgerForm } from './src/features/transactions/useLedgerForm';
import { TabBar } from './src/navigation/TabBar';
import type { Tab } from './src/shared/types/accounting';
import { styles } from './src/theme/styles';

export default function App() {
  const [tab, setTab] = useState<Tab>('home');
  const session = useSession();
  const dashboard = useDashboardData();
  const ledger = useLedgerForm(dashboard.accounts, dashboard.categories, dashboard.loadAll);

  useEffect(() => {
    if (session.authed) {
      dashboard.loadAll();
    }
  }, [session.authed]);

  async function handleLogin() {
    if (await session.signIn()) {
      await dashboard.loadAll();
    }
  }

  async function handleSaveTransaction() {
    if (await ledger.save()) {
      setTab('home');
    }
  }

  async function handleLogout() {
    await session.signOut();
    dashboard.clear();
  }

  async function handleCreateSimpleCategory() {
    await createSimpleCategory(`自定义${Date.now().toString().slice(-4)}`);
    await dashboard.loadAll();
  }

  async function handleCreateSimpleAccount() {
    await createSimpleAccount(`账户${Date.now().toString().slice(-4)}`);
    await dashboard.loadAll();
  }

  if (!session.ready) {
    return (
      <SafeAreaView style={styles.centered}>
        <ActivityIndicator />
      </SafeAreaView>
    );
  }

  if (!session.authed) {
    return (
      <SafeAreaView style={styles.container}>
        <LoginScreen
          username={session.username}
          password={session.password}
          error={session.error}
          onUsernameChange={session.setUsername}
          onPasswordChange={session.setPassword}
          onLogin={handleLogin}
        />
      </SafeAreaView>
    );
  }

  const error = dashboard.error || ledger.error;
  const message = ledger.message;

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar style="dark" />
      <ScrollView contentContainerStyle={styles.scroll}>
        <Text style={styles.title}>好好记账</Text>
        {!!error && <Text style={styles.error}>{error}</Text>}
        {!!message && <Text style={styles.success}>{message}</Text>}

        {tab === 'home' && <HomeScreen summary={dashboard.summary} transactions={dashboard.transactions} />}
        {tab === 'add' && (
          <AddTransactionScreen
            txType={ledger.txType}
            amount={ledger.amount}
            note={ledger.note}
            aiText={ledger.aiText}
            categories={ledger.filteredCategories}
            accounts={dashboard.accounts}
            categoryId={ledger.categoryId}
            accountId={ledger.accountId}
            onTxTypeChange={ledger.setTxType}
            onAmountChange={ledger.setAmount}
            onNoteChange={ledger.setNote}
            onAITextChange={ledger.setAiText}
            onCategoryChange={ledger.setCategoryId}
            onAccountChange={ledger.setAccountId}
            onSave={handleSaveTransaction}
            onAIParse={ledger.parseAI}
          />
        )}
        {tab === 'reports' && <ReportsScreen summary={dashboard.summary} />}
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
