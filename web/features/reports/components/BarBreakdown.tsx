import { formatMoney } from '@/lib/format';

type Item = {
  label: string;
  amount: number;
};

type Props = {
  eyebrow: string;
  title: string;
  badge: string;
  empty: string;
  items: Item[];
  expense?: boolean;
};

export function BarBreakdown({ eyebrow, title, badge, empty, items, expense }: Props) {
  const max = Math.max(...items.map((item) => item.amount), 1);

  return (
    <section className="card">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">{eyebrow}</span>
          <h3>{title}</h3>
        </div>
        <span className="badge">{badge}</span>
      </div>
      <div className="chart-bars">
        {items.length ? null : <div className="empty-state">{empty}</div>}
        {items.map((item) => (
          <div className="bar-row" key={item.label}>
            <div className="bar-row-header">
              <span>{item.label}</span>
              <span>{formatMoney(item.amount)}</span>
            </div>
            <div className="bar-track">
              <div
                className={expense ? 'bar-fill expense' : 'bar-fill'}
                style={{ width: `${Math.max(6, (item.amount / max) * 100)}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
