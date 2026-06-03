import { Text, View } from 'react-native';
import type { Summary } from '../../shared/types/accounting';
import { formatMoney } from '../../shared/utils/format';
import { styles } from '../../theme/styles';

export function ReportsScreen({ summary }: { summary: Summary | null }) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>报表</Text>
      <Text style={styles.muted}>收支趋势</Text>
      {(summary?.trend || summary?.monthlyTrend?.map((item) => ({ period: item.month, income: item.income, expense: item.expense })) || []).slice(-6).map((item) => (
        <Text key={item.period}>
          {item.period}: 收 {formatMoney(item.income)} / 支 {formatMoney(item.expense)}
        </Text>
      ))}
      <Text style={[styles.muted, { marginTop: 12 }]}>预算执行率</Text>
      {(summary?.budgetExecution || []).map((item) => (
        <Text key={`${item.month}-${item.categoryId}`}>
          {item.month} {item.category}: {Math.round(item.usageRate * 100)}% · 剩余 {formatMoney(item.remaining)}
        </Text>
      ))}
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
      <Text style={[styles.muted, { marginTop: 12 }]}>账户余额趋势</Text>
      {(summary?.accountBalanceTrend || []).slice(-6).map((item) => (
        <Text key={`${item.period}-${item.accountId}`}>
          {item.period} {item.account}: {formatMoney(item.balance)}
        </Text>
      ))}
      <Text style={[styles.muted, { marginTop: 12 }]}>月汇总</Text>
      {(summary?.monthlySummaries || []).slice(-6).map((item) => (
        <Text key={item.period}>
          {item.period}: 收 {formatMoney(item.income)} / 支 {formatMoney(item.expense)} / {item.txCount} 笔
        </Text>
      ))}
    </View>
  );
}
