import type { BudgetExecution } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function BudgetExecutionPanel({ items }: { items: BudgetExecution[] }) {
  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Budget</span>
          <h3>预算执行率</h3>
        </div>
        <span className="badge">{items.length} 项</span>
      </div>
      <div className="chart-bars">
        {items.length ? null : <div className="empty-state">暂无预算数据。</div>}
        {items.map((item) => (
          <div className="bar-row" key={`${item.budgetId}-${item.month}-${item.categoryId}`}>
            <div className="bar-row-header">
              <span>
                {item.month} · {item.category}
              </span>
              <span>
                {Math.round(item.usageRate * 100)}% · 剩余 {formatMoney(item.remaining)}
              </span>
            </div>
            <div className="bar-track">
              <div className="bar-fill expense" style={{ width: `${Math.min(100, Math.max(4, item.usageRate * 100))}%` }} />
            </div>
            <span className="muted">
              已用 {formatMoney(item.expense)} / 预算 {formatMoney(item.budget)}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
