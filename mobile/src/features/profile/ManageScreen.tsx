import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';
import { formatMoney } from '../../shared/utils/format';
import { styles } from '../../theme/styles';

type Props = {
  accounts: Account[];
  categories: Category[];
  accountName: string;
  accountType: string;
  categoryName: string;
  categoryType: TransactionType;
  editingAccountId: number;
  editingCategoryId: number;
  onAccountNameChange: (value: string) => void;
  onAccountTypeChange: (value: string) => void;
  onCategoryNameChange: (value: string) => void;
  onCategoryTypeChange: (value: TransactionType) => void;
  onSaveAccount: () => void;
  onSaveCategory: () => void;
  onEditAccount: (account: Account) => void;
  onEditCategory: (category: Category) => void;
  onDeleteAccount: (account: Account) => void;
  onDeleteCategory: (category: Category) => void;
  onCancelAccountEdit: () => void;
  onCancelCategoryEdit: () => void;
};

export function ManageScreen({
  accounts,
  categories,
  accountName,
  accountType,
  categoryName,
  categoryType,
  editingAccountId,
  editingCategoryId,
  onAccountNameChange,
  onAccountTypeChange,
  onCategoryNameChange,
  onCategoryTypeChange,
  onSaveAccount,
  onSaveCategory,
  onEditAccount,
  onEditCategory,
  onDeleteAccount,
  onDeleteCategory,
  onCancelAccountEdit,
  onCancelCategoryEdit,
}: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>账户管理</Text>
      <TextInput style={styles.input} placeholder="账户名称" value={accountName} onChangeText={onAccountNameChange} />
      <View style={styles.row}>
        {['cash', 'bank', 'alipay', 'wechat', 'custom'].map((item) => (
          <TouchableOpacity key={item} style={[styles.chip, accountType === item && styles.chipActive]} onPress={() => onAccountTypeChange(item)}>
            <Text style={accountType === item ? styles.chipTextActive : styles.chipText}>{item}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TouchableOpacity style={styles.primaryBtn} onPress={onSaveAccount}>
        <Text style={styles.primaryBtnText}>{editingAccountId ? '保存账户' : '新增账户'}</Text>
      </TouchableOpacity>
      {editingAccountId ? (
        <TouchableOpacity style={styles.secondaryBtn} onPress={onCancelAccountEdit}>
          <Text style={styles.secondaryBtnText}>取消账户编辑</Text>
        </TouchableOpacity>
      ) : null}
      {accounts.map((item) => (
        <View key={item.id} style={styles.listItem}>
          <Text style={{ fontWeight: '700' }}>{item.name}</Text>
          <Text>{formatMoney(item.balance)} · {item.type}</Text>
          <View style={styles.row}>
            <TouchableOpacity style={styles.secondaryBtnCompact} onPress={() => onEditAccount(item)}>
              <Text style={styles.secondaryBtnText}>编辑</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.dangerBtnCompact} onPress={() => onDeleteAccount(item)}>
              <Text style={styles.dangerBtnText}>删除</Text>
            </TouchableOpacity>
          </View>
        </View>
      ))}

      <Text style={styles.sectionTitle}>分类管理</Text>
      <TextInput style={styles.input} placeholder="分类名称" value={categoryName} onChangeText={onCategoryNameChange} />
      <View style={styles.row}>
        {(['expense', 'income'] as const).map((item) => (
          <TouchableOpacity key={item} style={[styles.chip, categoryType === item && styles.chipActive]} onPress={() => onCategoryTypeChange(item)}>
            <Text style={categoryType === item ? styles.chipTextActive : styles.chipText}>{item === 'expense' ? '支出' : '收入'}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TouchableOpacity style={styles.primaryBtn} onPress={onSaveCategory}>
        <Text style={styles.primaryBtnText}>{editingCategoryId ? '保存分类' : '新增分类'}</Text>
      </TouchableOpacity>
      {editingCategoryId ? (
        <TouchableOpacity style={styles.secondaryBtn} onPress={onCancelCategoryEdit}>
          <Text style={styles.secondaryBtnText}>取消分类编辑</Text>
        </TouchableOpacity>
      ) : null}
      {categories.map((item) => (
        <View key={item.id} style={styles.listItem}>
          <Text style={{ fontWeight: '700' }}>{item.name}</Text>
          <Text>{item.type === 'expense' ? '支出' : '收入'} · {item.isSystem ? '系统分类' : '自定义分类'}</Text>
          <View style={styles.row}>
            <TouchableOpacity style={styles.secondaryBtnCompact} disabled={item.isSystem} onPress={() => onEditCategory(item)}>
              <Text style={styles.secondaryBtnText}>编辑</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.dangerBtnCompact} disabled={item.isSystem} onPress={() => onDeleteCategory(item)}>
              <Text style={styles.dangerBtnText}>删除</Text>
            </TouchableOpacity>
          </View>
        </View>
      ))}
    </View>
  );
}
