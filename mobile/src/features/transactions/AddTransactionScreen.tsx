import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';
import { styles } from '../../theme/styles';

type Props = {
  txType: TransactionType;
  amount: string;
  note: string;
  occurredAt: string;
  aiText: string;
  categories: Category[];
  accounts: Account[];
  categoryId: number;
  accountId: number;
  onTxTypeChange: (value: TransactionType) => void;
  onAmountChange: (value: string) => void;
  onNoteChange: (value: string) => void;
  onOccurredAtChange: (value: string) => void;
  onAITextChange: (value: string) => void;
  onCategoryChange: (value: number) => void;
  onAccountChange: (value: number) => void;
  onSave: () => void;
  onCancelEdit?: () => void;
  onAIParse: () => void;
  editing?: boolean;
};

export function AddTransactionScreen({
  txType,
  amount,
  note,
  occurredAt,
  aiText,
  categories,
  accounts,
  categoryId,
  accountId,
  onTxTypeChange,
  onAmountChange,
  onNoteChange,
  onOccurredAtChange,
  onAITextChange,
  onCategoryChange,
  onAccountChange,
  onSave,
  onCancelEdit,
  onAIParse,
  editing = false,
}: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>{editing ? '编辑账单' : '记一笔'}</Text>
      <View style={styles.row}>
        {(['expense', 'income'] as const).map((item) => (
          <TouchableOpacity key={item} style={[styles.chip, txType === item && styles.chipActive]} onPress={() => onTxTypeChange(item)}>
            <Text style={txType === item ? styles.chipTextActive : styles.chipText}>{item === 'expense' ? '支出' : '收入'}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TextInput style={styles.input} keyboardType="numeric" placeholder="金额" value={amount} onChangeText={onAmountChange} />
      <TextInput style={styles.input} placeholder="发生时间，留空为当前时间" value={occurredAt} onChangeText={onOccurredAtChange} />
      <Text style={styles.muted}>分类</Text>
      <View style={styles.row}>
        {categories.map((item) => (
          <TouchableOpacity key={item.id} style={[styles.chip, categoryId === item.id && styles.chipActive]} onPress={() => onCategoryChange(item.id)}>
            <Text style={categoryId === item.id ? styles.chipTextActive : styles.chipText}>{item.name}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <Text style={styles.muted}>账户</Text>
      <View style={styles.row}>
        {accounts.map((item) => (
          <TouchableOpacity key={item.id} style={[styles.chip, accountId === item.id && styles.chipActive]} onPress={() => onAccountChange(item.id)}>
            <Text style={accountId === item.id ? styles.chipTextActive : styles.chipText}>{item.name}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TextInput style={styles.input} placeholder="备注" value={note} onChangeText={onNoteChange} />
      <TouchableOpacity style={styles.primaryBtn} onPress={onSave}>
        <Text style={styles.primaryBtnText}>{editing ? '保存修改' : '保存账单'}</Text>
      </TouchableOpacity>
      {editing && onCancelEdit ? (
        <TouchableOpacity style={styles.secondaryBtn} onPress={onCancelEdit}>
          <Text style={styles.secondaryBtnText}>取消编辑</Text>
        </TouchableOpacity>
      ) : null}

      <Text style={styles.sectionTitle}>AI 对话记账</Text>
      <TextInput style={[styles.input, { height: 80 }]} multiline value={aiText} onChangeText={onAITextChange} />
      <TouchableOpacity style={styles.secondaryBtn} onPress={onAIParse}>
        <Text style={styles.secondaryBtnText}>AI 解析</Text>
      </TouchableOpacity>
    </View>
  );
}
