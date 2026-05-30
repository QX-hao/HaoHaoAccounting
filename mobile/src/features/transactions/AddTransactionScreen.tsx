import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { Account, Category, TransactionType } from '../../shared/types/accounting';
import { styles } from '../../theme/styles';

type Props = {
  txType: TransactionType;
  amount: string;
  note: string;
  aiText: string;
  categoryName: string;
  accountName: string;
  onTxTypeChange: (value: TransactionType) => void;
  onAmountChange: (value: string) => void;
  onNoteChange: (value: string) => void;
  onAITextChange: (value: string) => void;
  onSave: () => void;
  onAIParse: () => void;
};

export function AddTransactionScreen({
  txType,
  amount,
  note,
  aiText,
  categoryName,
  accountName,
  onTxTypeChange,
  onAmountChange,
  onNoteChange,
  onAITextChange,
  onSave,
  onAIParse,
}: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>记一笔</Text>
      <View style={styles.row}>
        {(['expense', 'income'] as const).map((item) => (
          <TouchableOpacity key={item} style={[styles.chip, txType === item && styles.chipActive]} onPress={() => onTxTypeChange(item)}>
            <Text style={txType === item ? styles.chipTextActive : styles.chipText}>{item === 'expense' ? '支出' : '收入'}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TextInput style={styles.input} keyboardType="numeric" placeholder="金额" value={amount} onChangeText={onAmountChange} />
      <Text style={styles.muted}>分类：{categoryName || '-'}</Text>
      <Text style={styles.muted}>账户：{accountName || '-'}</Text>
      <TextInput style={styles.input} placeholder="备注" value={note} onChangeText={onNoteChange} />
      <TouchableOpacity style={styles.primaryBtn} onPress={onSave}>
        <Text style={styles.primaryBtnText}>保存账单</Text>
      </TouchableOpacity>

      <Text style={styles.sectionTitle}>AI 对话记账</Text>
      <TextInput style={[styles.input, { height: 80 }]} multiline value={aiText} onChangeText={onAITextChange} />
      <TouchableOpacity style={styles.secondaryBtn} onPress={onAIParse}>
        <Text style={styles.secondaryBtnText}>AI 解析</Text>
      </TouchableOpacity>
    </View>
  );
}
