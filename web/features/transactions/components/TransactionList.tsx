import type { Transaction } from '@/lib/types';
import { formatDateTime, formatMoney } from '@/lib/format';

export function TransactionList({ transactions }: { transactions: Transaction[] }) {
  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 12 }}>
        <div>
          <span className="eyebrow">Ledger</span>
          <h3>账单列表</h3>
        </div>
        <span className="badge">{transactions.length} 条</span>
      </div>
      <div className="list">
        {transactions.length === 0 ? <div className="empty-state">暂无账单。</div> : null}
        {transactions.map((row) => (
          <div className="transaction-row" key={row.id}>
            <div className="row-main">
              <span className={`icon-badge ${row.type}`} aria-hidden="true">
                {row.category?.name?.slice(0, 1) || '账'}
              </span>
              <div className="row-title">
                <strong>{row.note || row.category?.name || '未命名账单'}</strong>
                <small>
                  {formatDateTime(row.occurredAt)} · {row.category?.name || '-'} · {row.account?.name || '-'}
                  {row.tags ? ` · ${row.tags}` : ''}
                </small>
              </div>
            </div>
            <span className={`amount ${row.type}`}>
              {formatMoney(row.type === 'income' ? row.amount : -row.amount, { signed: true })}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
