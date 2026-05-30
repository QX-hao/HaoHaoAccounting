import type { Summary } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function SummaryCards({ summary }: { summary: Summary | null }) {
  return (
    <div className="grid three">
      <section className="card stat-card">
        <span className="label">本期收入</span>
        <span className="value amount income">{formatMoney(summary?.income)}</span>
        <span className="hint">所有收入分类合计。</span>
      </section>
      <section className="card stat-card">
        <span className="label">本期支出</span>
        <span className="value amount expense">{formatMoney(summary?.expense)}</span>
        <span className="hint">所有支出分类合计。</span>
      </section>
      <section className="card stat-card">
        <span className="label">本期结余</span>
        <span className="value">{formatMoney(summary?.balance)}</span>
        <span className="hint">收入减支出后的可用金额。</span>
      </section>
    </div>
  );
}
