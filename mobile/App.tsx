import { useEffect, useState } from 'react';
import { ActivityIndicator, Alert, SafeAreaView, ScrollView, Text } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import * as Clipboard from 'expo-clipboard';
import * as DocumentPicker from 'expo-document-picker';
import * as FileSystem from 'expo-file-system';
import * as Sharing from 'expo-sharing';
import { LoginScreen } from './src/features/auth/LoginScreen';
import { useSession } from './src/features/auth/useSession';
import { useDashboardData } from './src/features/dashboard/useDashboardData';
import { DataIOScreen } from './src/features/dataio/DataIOScreen';
import {
  exportCSVText,
  importText,
  previewImportFile,
  previewImportText,
  startImportFileJob,
  type ImportFileAsset,
} from './src/features/dataio/api';
import { HomeScreen } from './src/features/home/HomeScreen';
import { ManageScreen } from './src/features/profile/ManageScreen';
import {
  createAccount,
  createCategory,
  createSimpleAccount,
  createSimpleCategory,
  deleteAccount,
  deleteCategory,
  updateAccount,
  updateCategory,
} from './src/features/profile/api';
import { ProfileScreen } from './src/features/profile/ProfileScreen';
import { ReportsScreen } from './src/features/reports/ReportsScreen';
import { AddTransactionScreen } from './src/features/transactions/AddTransactionScreen';
import { listTransactions, deleteTransaction, type TransactionFilters } from './src/features/transactions/api';
import { TransactionsScreen } from './src/features/transactions/TransactionsScreen';
import { useLedgerForm } from './src/features/transactions/useLedgerForm';
import { TabBar } from './src/navigation/TabBar';
import type { Account, Category, ImportPreview, Tab, Transaction, TransactionType } from './src/shared/types/accounting';
import { styles } from './src/theme/styles';

export default function App() {
  const [tab, setTab] = useState<Tab>('home');
  const [txFilters, setTxFilters] = useState<TransactionFilters>({ page: 1, pageSize: 20, type: '' });
  const [txDraftType, setTxDraftType] = useState<'' | TransactionType>('');
  const [txDraftCategoryId, setTxDraftCategoryId] = useState(0);
  const [txDraftAccountId, setTxDraftAccountId] = useState(0);
  const [txDraftStart, setTxDraftStart] = useState('');
  const [txDraftEnd, setTxDraftEnd] = useState('');
  const [txDraftKeyword, setTxDraftKeyword] = useState('');
  const [txItems, setTxItems] = useState<Transaction[]>([]);
  const [txTotal, setTxTotal] = useState(0);
  const [accountName, setAccountName] = useState('');
  const [accountType, setAccountType] = useState('custom');
  const [editingAccount, setEditingAccount] = useState<Account | null>(null);
  const [categoryName, setCategoryName] = useState('');
  const [categoryType, setCategoryType] = useState<TransactionType>('expense');
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [csvText, setCsvText] = useState('');
  const [exportText, setExportText] = useState('');
  const [selectedImportFile, setSelectedImportFile] = useState('');
  const [selectedImportAsset, setSelectedImportAsset] = useState<ImportFileAsset | null>(null);
  const [exportFileUri, setExportFileUri] = useState('');
  const [importPreview, setImportPreview] = useState<ImportPreview | null>(null);
  const [appNotice, setAppNotice] = useState('');
  const session = useSession();
  const dashboard = useDashboardData();
  const ledger = useLedgerForm(dashboard.accounts, dashboard.categories, dashboard.loadAll);

  useEffect(() => {
    if (session.authed) {
      dashboard.loadAll();
    }
  }, [session.authed]);

  useEffect(() => {
    if (session.authed && tab === 'transactions') {
      loadTransactions(txFilters);
    }
  }, [session.authed, tab, txFilters]);

  async function handleLogin() {
    if (await session.signIn()) {
      await dashboard.loadAll();
    }
  }

  async function handleSaveTransaction() {
    if (await ledger.save()) {
      setTab('home');
      await loadTransactions(txFilters);
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

  async function loadTransactions(filters: TransactionFilters) {
    try {
      setAppNotice('');
      const resp = await listTransactions(filters);
      setTxItems(resp.items || []);
      setTxTotal(resp.pagination?.total ?? resp.items?.length ?? 0);
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '账单加载失败');
    }
  }

  function applyTransactionFilters() {
    setTxFilters({
      page: 1,
      pageSize: 20,
      start: txDraftStart,
      end: txDraftEnd,
      type: txDraftType,
      categoryId: txDraftCategoryId,
      accountId: txDraftAccountId,
      q: txDraftKeyword,
    });
  }

  function editTransaction(transaction: Transaction) {
    ledger.startEdit(transaction);
    setTab('add');
  }

  async function removeTransaction(transaction: Transaction) {
    Alert.alert('删除账单', `确定删除「${transaction.note || transaction.category?.name || transaction.id}」吗？`, [
      { text: '取消', style: 'cancel' },
      {
        text: '删除',
        style: 'destructive',
        onPress: async () => {
          try {
            await deleteTransaction(transaction.id);
            await dashboard.loadAll();
            await loadTransactions(txFilters);
          } catch (err) {
            dashboard.setError(err instanceof Error ? err.message : '删除失败');
          }
        },
      },
    ]);
  }

  async function saveAccount() {
    try {
      setAppNotice('');
      if (!accountName.trim()) {
        dashboard.setError('请输入账户名称');
        return;
      }
      if (editingAccount) {
        await updateAccount(editingAccount.id, { name: accountName.trim(), type: accountType, balance: editingAccount.balance });
      } else {
        await createAccount({ name: accountName.trim(), type: accountType, balance: 0 });
      }
      resetAccountForm();
      await dashboard.loadAll();
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '账户保存失败');
    }
  }

  async function saveCategory() {
    try {
      setAppNotice('');
      if (!categoryName.trim()) {
        dashboard.setError('请输入分类名称');
        return;
      }
      if (editingCategory) {
        await updateCategory(editingCategory.id, { name: categoryName.trim(), type: categoryType });
      } else {
        await createCategory({ name: categoryName.trim(), type: categoryType });
      }
      resetCategoryForm();
      await dashboard.loadAll();
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '分类保存失败');
    }
  }

  function startEditAccount(account: Account) {
    setEditingAccount(account);
    setAccountName(account.name);
    setAccountType(account.type);
  }

  function startEditCategory(category: Category) {
    if (category.isSystem) return;
    setEditingCategory(category);
    setCategoryName(category.name);
    setCategoryType(category.type);
  }

  async function removeAccount(account: Account) {
    Alert.alert('删除账户', `确定删除「${account.name}」吗？`, [
      { text: '取消', style: 'cancel' },
      {
        text: '删除',
        style: 'destructive',
        onPress: async () => {
          try {
            await deleteAccount(account.id);
            if (editingAccount?.id === account.id) resetAccountForm();
            await dashboard.loadAll();
          } catch (err) {
            dashboard.setError(err instanceof Error ? err.message : '账户删除失败');
          }
        },
      },
    ]);
  }

  async function removeCategory(category: Category) {
    if (category.isSystem) return;
    Alert.alert('删除分类', `确定删除「${category.name}」吗？`, [
      { text: '取消', style: 'cancel' },
      {
        text: '删除',
        style: 'destructive',
        onPress: async () => {
          try {
            await deleteCategory(category.id);
            if (editingCategory?.id === category.id) resetCategoryForm();
            await dashboard.loadAll();
          } catch (err) {
            dashboard.setError(err instanceof Error ? err.message : '分类删除失败');
          }
        },
      },
    ]);
  }

  function resetAccountForm() {
    setEditingAccount(null);
    setAccountName('');
    setAccountType('custom');
  }

  function resetCategoryForm() {
    setEditingCategory(null);
    setCategoryName('');
    setCategoryType('expense');
  }

  async function pickImportFile() {
    try {
      setAppNotice('');
      const result = await DocumentPicker.getDocumentAsync({
        copyToCacheDirectory: true,
        multiple: false,
        type: [
          'text/csv',
          'text/comma-separated-values',
          'application/vnd.ms-excel',
          'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          'text/plain',
        ],
      });
      if (result.canceled || !result.assets[0]) return;

      const asset = result.assets[0];
      setSelectedImportFile(asset.name);
      setSelectedImportAsset({ uri: asset.uri, name: asset.name, mimeType: asset.mimeType });
      setImportPreview(null);
      dashboard.setError('');
      if (isTextImportFile(asset.name, asset.mimeType)) {
        const content = await new FileSystem.File(asset.uri).text();
        setCsvText(content);
        setAppNotice(`已读取文件：${asset.name}`);
        return;
      }
      setCsvText('');
      setAppNotice(`已选择文件：${asset.name}，可直接上传预览或后台导入`);
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '读取文件失败');
    }
  }

  async function previewCSV() {
    try {
      setAppNotice('');
      const resp = selectedImportAsset ? await previewImportFile(selectedImportAsset) : await previewImportText(csvText);
      setImportPreview(resp);
      dashboard.setError('');
      setAppNotice(`预览完成：有效 ${resp.validRows} 条，重复 ${resp.duplicateRows} 条，失败 ${resp.failedRows} 条`);
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '预览失败');
    }
  }

  async function importCSV() {
    try {
      setAppNotice('');
      if (selectedImportAsset) {
        const job = await startImportFileJob(selectedImportAsset);
        setImportPreview(null);
        await dashboard.loadAll();
        dashboard.setError('');
        setAppNotice(`已创建后台导入任务 #${job.id}：${job.status}`);
        return;
      }
      const resp = await importText(csvText);
      setImportPreview(null);
      await dashboard.loadAll();
      dashboard.setError('');
      setAppNotice(`导入完成：成功 ${resp.success} 条，跳过 ${resp.skipped} 条，失败 ${resp.failed} 条`);
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '导入失败');
    }
  }

  async function exportCSV() {
    try {
      setAppNotice('');
      const text = await exportCSVText();
      setExportText(text);
      setExportFileUri('');
      dashboard.setError('');
      setAppNotice('导出文本已生成');
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '导出失败');
    }
  }

  async function copyExportText() {
    if (!exportText) return;
    await Clipboard.setStringAsync(exportText);
    setAppNotice('导出内容已复制');
  }

  async function shareExportCSV() {
    if (!exportText) return;
    try {
      const file = exportFileUri
        ? new FileSystem.File(exportFileUri)
        : new FileSystem.File(FileSystem.Paths.cache, `haohao-transactions-${Date.now()}.csv`);
      if (!exportFileUri) {
        file.write(exportText);
        setExportFileUri(file.uri);
      }
      const available = await Sharing.isAvailableAsync();
      if (!available) {
        dashboard.setError('当前设备不支持系统分享');
        return;
      }
      await Sharing.shareAsync(file.uri, {
        mimeType: 'text/csv',
        dialogTitle: '分享好好记账 CSV',
        UTI: 'public.comma-separated-values-text',
      });
      setAppNotice('已打开系统分享');
    } catch (err) {
      dashboard.setError(err instanceof Error ? err.message : '分享失败');
    }
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
  const message = ledger.message || appNotice;

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
            occurredAt={ledger.occurredAt}
            aiText={ledger.aiText}
            categories={ledger.filteredCategories}
            accounts={dashboard.accounts}
            categoryId={ledger.categoryId}
            accountId={ledger.accountId}
            onTxTypeChange={ledger.setTxType}
            onAmountChange={ledger.setAmount}
            onNoteChange={ledger.setNote}
            onOccurredAtChange={ledger.setOccurredAt}
            onAITextChange={ledger.setAiText}
            onCategoryChange={ledger.setCategoryId}
            onAccountChange={ledger.setAccountId}
            onSave={handleSaveTransaction}
            onCancelEdit={ledger.resetForm}
            onAIParse={ledger.parseAI}
            editing={Boolean(ledger.editing)}
          />
        )}
        {tab === 'transactions' && (
          <TransactionsScreen
            transactions={txItems}
            accounts={dashboard.accounts}
            categories={dashboard.categories}
            page={txFilters.page}
            total={txTotal}
            type={txDraftType}
            categoryId={txDraftCategoryId}
            accountId={txDraftAccountId}
            start={txDraftStart}
            end={txDraftEnd}
            keyword={txDraftKeyword}
            onTypeChange={setTxDraftType}
            onCategoryChange={setTxDraftCategoryId}
            onAccountChange={setTxDraftAccountId}
            onStartChange={setTxDraftStart}
            onEndChange={setTxDraftEnd}
            onKeywordChange={setTxDraftKeyword}
            onApplyFilters={applyTransactionFilters}
            onPageChange={(page) => setTxFilters((current) => ({ ...current, page }))}
            onEdit={editTransaction}
            onDelete={removeTransaction}
          />
        )}
        {tab === 'manage' && (
          <ManageScreen
            accounts={dashboard.accounts}
            categories={dashboard.categories}
            accountName={accountName}
            accountType={accountType}
            categoryName={categoryName}
            categoryType={categoryType}
            editingAccountId={editingAccount?.id || 0}
            editingCategoryId={editingCategory?.id || 0}
            onAccountNameChange={setAccountName}
            onAccountTypeChange={setAccountType}
            onCategoryNameChange={setCategoryName}
            onCategoryTypeChange={setCategoryType}
            onSaveAccount={saveAccount}
            onSaveCategory={saveCategory}
            onEditAccount={startEditAccount}
            onEditCategory={startEditCategory}
            onDeleteAccount={removeAccount}
            onDeleteCategory={removeCategory}
            onCancelAccountEdit={resetAccountForm}
            onCancelCategoryEdit={resetCategoryForm}
          />
        )}
        {tab === 'io' && (
          <DataIOScreen
            csvText={csvText}
            exportText={exportText}
            selectedImportFile={selectedImportFile}
            preview={importPreview}
            onCSVTextChange={(value) => {
              setCsvText(value);
              setSelectedImportFile('');
              setSelectedImportAsset(null);
              setImportPreview(null);
            }}
            onPickFile={pickImportFile}
            onPreview={previewCSV}
            onImport={importCSV}
            onExport={exportCSV}
            onCopyExport={copyExportText}
            onShareExport={shareExportCSV}
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

function isTextImportFile(name: string, mimeType?: string) {
  const lowerName = name.toLowerCase();
  const lowerMime = (mimeType || '').toLowerCase();
  return (
    lowerName.endsWith('.csv') ||
    lowerName.endsWith('.txt') ||
    lowerMime === 'text/csv' ||
    lowerMime === 'text/comma-separated-values' ||
    lowerMime === 'text/plain'
  );
}
