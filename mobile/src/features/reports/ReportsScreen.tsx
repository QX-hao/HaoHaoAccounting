import { Text, View } from 'react-native';
import type { Summary } from '../../shared/types/accounting';
import { formatMoney } from '../../shared/utils/format';
import { styles } from '../../theme/styles';

export function ReportsScreen({ summary }: { summary: Summary | null }) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>报表</Text>
      <Text style={styles.muted}>分类占比（支出）</Text>
      {summary?.byCategory?.map((item) => (
        <Text key={item.category}>
          {item.category}: {formatMoney(item.amount)}
        </Text>
      ))}
      <Text style={[styles.muted, { marginTop: 12 }]}>按账户统计（支出）</Text>
      {summary?.byAccount?.map((item) => (
        <Text key={item.account}>
          {item.account}: {formatMoney(item.amount)}
        </Text>
      ))}
    </View>
  );
}
