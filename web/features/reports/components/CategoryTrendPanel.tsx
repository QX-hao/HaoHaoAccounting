import type { CategoryTrendPoint } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function CategoryTrendPanel({ items }: { items: CategoryTrendPoint[] }) {
  const rows = items.slice(0, 12);

  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Compare</span>
          <h3>分类趋势对比</h3>
        </div>
        <span className="badge">Top {rows.length}</span>
      </div>
      <div className="list">
        {rows.length ? null : <div className="empty-state">暂无分类趋势。</div>}
        {rows.map((item) => (
          <div className="list-row" key={`${item.period}-${item.categoryId}`}>
            <div className="row-title">
              <strong>{item.category}</strong>
              <small>{item.period}</small>
            </div>
            <span className="amount expense">{formatMoney(item.amount)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
