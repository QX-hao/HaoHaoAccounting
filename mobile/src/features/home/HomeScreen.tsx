import { Text, View } from 'react-native';
import type { Summary, Transaction } from '../../shared/types/accounting';
import { formatMoney, transactionTypeLabel } from '../../shared/utils/format';
import { styles } from '../../theme/styles';

export function HomeScreen({ summary, transactions }: { summary: Summary | null; transactions: Transaction[] }) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>本月概览</Text>
      <Text>收入：{formatMoney(summary?.income)}</Text>
      <Text>支出：{formatMoney(summary?.expense)}</Text>
      <Text>结余：{formatMoney(summary?.balance)}</Text>
      <Text style={[styles.sectionTitle, { marginTop: 12 }]}>最近账单</Text>
      {transactions.map((item) => (
        <View key={item.id} style={styles.listItem}>
          <Text>{item.note}</Text>
          <Text>
            {transactionTypeLabel(item.type)} {formatMoney(item.amount)} · {item.category?.name}
          </Text>
        </View>
      ))}
    </View>
  );
}
