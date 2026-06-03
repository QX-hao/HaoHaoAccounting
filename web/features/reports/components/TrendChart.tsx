import type { TrendPoint } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function TrendChart({ items, granularity }: { items: TrendPoint[]; granularity: string }) {
  const max = Math.max(...items.flatMap((item) => [item.income, item.expense]), 1);

  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Trend</span>
          <h3>收支趋势</h3>
        </div>
        <span className="badge">{granularityLabel(granularity)}</span>
      </div>
      <div className="chart-bars">
        {items.length ? null : <div className="empty-state">暂无趋势数据。</div>}
        {items.map((item) => (
          <div className="bar-row" key={item.period}>
            <div className="bar-row-header">
              <span>{item.period}</span>
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

function granularityLabel(value: string) {
  if (value === 'day') return '按天';
  if (value === 'week') return '按周';
  return '按月';
}
