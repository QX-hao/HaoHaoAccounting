import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { Account, Category, Transaction, TransactionType } from '../../shared/types/accounting';
import { formatMoney, transactionTypeLabel } from '../../shared/utils/format';
import { styles } from '../../theme/styles';

type Props = {
  transactions: Transaction[];
  accounts: Account[];
  categories: Category[];
  page: number;
  total: number;
  type: '' | TransactionType;
  categoryId: number;
  accountId: number;
  start: string;
  end: string;
  keyword: string;
  onTypeChange: (value: '' | TransactionType) => void;
  onCategoryChange: (value: number) => void;
  onAccountChange: (value: number) => void;
  onStartChange: (value: string) => void;
  onEndChange: (value: string) => void;
  onKeywordChange: (value: string) => void;
  onApplyFilters: () => void;
  onPageChange: (page: number) => void;
  onEdit: (transaction: Transaction) => void;
  onDelete: (transaction: Transaction) => void;
};

export function TransactionsScreen({
  transactions,
  accounts,
  categories,
  page,
  total,
  type,
  categoryId,
  accountId,
  start,
  end,
  keyword,
  onTypeChange,
  onCategoryChange,
  onAccountChange,
  onStartChange,
  onEndChange,
  onKeywordChange,
  onApplyFilters,
  onPageChange,
  onEdit,
  onDelete,
}: Props) {
  const totalPages = Math.max(1, Math.ceil(total / 20));
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>账单筛选</Text>
      <View style={styles.row}>
        {[
          ['', '全部'],
          ['expense', '支出'],
          ['income', '收入'],
        ].map(([key, label]) => (
          <TouchableOpacity key={key} style={[styles.chip, type === key && styles.chipActive]} onPress={() => onTypeChange(key as '' | TransactionType)}>
            <Text style={type === key ? styles.chipTextActive : styles.chipText}>{label}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <Text style={styles.muted}>日期</Text>
      <View style={styles.row}>
        <TextInput style={[styles.input, styles.inputHalf]} placeholder="开始 YYYY-MM-DD" value={start} onChangeText={onStartChange} />
        <TextInput style={[styles.input, styles.inputHalf]} placeholder="结束 YYYY-MM-DD" value={end} onChangeText={onEndChange} />
      </View>
      <Text style={styles.muted}>分类</Text>
      <View style={styles.row}>
        <TouchableOpacity style={[styles.chip, categoryId === 0 && styles.chipActive]} onPress={() => onCategoryChange(0)}>
          <Text style={categoryId === 0 ? styles.chipTextActive : styles.chipText}>全部</Text>
        </TouchableOpacity>
        {categories.map((item) => (
          <TouchableOpacity key={item.id} style={[styles.chip, categoryId === item.id && styles.chipActive]} onPress={() => onCategoryChange(item.id)}>
            <Text style={categoryId === item.id ? styles.chipTextActive : styles.chipText}>{item.name}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <Text style={styles.muted}>账户</Text>
      <View style={styles.row}>
        <TouchableOpacity style={[styles.chip, accountId === 0 && styles.chipActive]} onPress={() => onAccountChange(0)}>
          <Text style={accountId === 0 ? styles.chipTextActive : styles.chipText}>全部</Text>
        </TouchableOpacity>
        {accounts.map((item) => (
          <TouchableOpacity key={item.id} style={[styles.chip, accountId === item.id && styles.chipActive]} onPress={() => onAccountChange(item.id)}>
            <Text style={accountId === item.id ? styles.chipTextActive : styles.chipText}>{item.name}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TextInput style={styles.input} placeholder="搜索备注或标签" value={keyword} onChangeText={onKeywordChange} />
      <TouchableOpacity style={styles.secondaryBtn} onPress={onApplyFilters}>
        <Text style={styles.secondaryBtnText}>应用筛选</Text>
      </TouchableOpacity>

      <Text style={styles.sectionTitle}>账单列表 · 共 {total} 条</Text>
      {transactions.map((item) => (
        <View key={item.id} style={styles.listItem}>
          <Text style={{ fontWeight: '700' }}>{item.note || item.category?.name || '未命名账单'}</Text>
          <Text>
            {transactionTypeLabel(item.type)} {formatMoney(item.amount)} · {item.category?.name} · {item.account?.name}
          </Text>
          <View style={styles.row}>
            <TouchableOpacity style={styles.secondaryBtnCompact} onPress={() => onEdit(item)}>
              <Text style={styles.secondaryBtnText}>编辑</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.dangerBtnCompact} onPress={() => onDelete(item)}>
              <Text style={styles.dangerBtnText}>删除</Text>
            </TouchableOpacity>
          </View>
        </View>
      ))}
      {transactions.length === 0 ? <Text style={styles.muted}>暂无账单</Text> : null}
      <View style={styles.row}>
        <TouchableOpacity style={styles.secondaryBtnCompact} disabled={page <= 1} onPress={() => onPageChange(page - 1)}>
          <Text style={styles.secondaryBtnText}>上一页</Text>
        </TouchableOpacity>
        <Text style={styles.muted}>
          {page} / {totalPages}
        </Text>
        <TouchableOpacity style={styles.secondaryBtnCompact} disabled={page >= totalPages} onPress={() => onPageChange(page + 1)}>
          <Text style={styles.secondaryBtnText}>下一页</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}
