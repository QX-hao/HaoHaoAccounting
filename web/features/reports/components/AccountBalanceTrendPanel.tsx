import type { AccountBalancePoint } from '@/lib/types';
import { formatMoney } from '@/lib/format';

export function AccountBalanceTrendPanel({ items }: { items: AccountBalancePoint[] }) {
  const rows = items.slice(-12);

  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Balance</span>
          <h3>账户余额变化</h3>
        </div>
        <span className="badge">{items.length} 点</span>
      </div>
      <div className="list">
        {rows.length ? null : <div className="empty-state">暂无账户余额趋势。</div>}
        {rows.map((item) => (
          <div className="list-row" key={`${item.period}-${item.accountId}`}>
            <div className="row-title">
              <strong>{item.account}</strong>
              <small>
                {item.period} · 净变化 {formatMoney(item.net)}
              </small>
            </div>
            <span className="amount">{formatMoney(item.balance)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
