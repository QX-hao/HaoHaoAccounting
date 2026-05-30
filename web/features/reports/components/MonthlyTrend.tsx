import type { MonthTrend as MonthTrendItem } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function MonthlyTrend({ items }: { items: MonthTrendItem[] }) {
  const max = Math.max(...items.flatMap((item) => [item.income, item.expense]), 1);

  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Trend</span>
          <h3>月度收支趋势</h3>
        </div>
        <span className="badge">{items.length} 个月</span>
      </div>
      <div className="chart-bars">
        {items.length ? null : <div className="empty-state">暂无月度趋势。</div>}
        {items.map((item) => (
          <div className="bar-row" key={item.month}>
            <div className="bar-row-header">
              <span>{item.month}</span>
              <span>
                收 {formatMoney(item.income)} / 支 {formatMoney(item.expense)}
              </span>
            </div>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${Math.max(4, (item.income / max) * 100)}%` }} />
            </div>
            <div className="bar-track">
              <div className="bar-fill expense" style={{ width: `${Math.max(4, (item.expense / max) * 100)}%` }} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
