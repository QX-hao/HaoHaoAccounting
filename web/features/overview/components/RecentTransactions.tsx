import Link from 'next/link';
import type { Transaction } from '@/lib/types';
import { formatDateTime, formatMoney, transactionTypeLabel } from '@/lib/format';

export function RecentTransactions({ items }: { items: Transaction[] }) {
  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 12 }}>
        <div>
          <span className="eyebrow">Recent</span>
          <h3>最近账单</h3>
        </div>
        <Link className="secondary" href="/transactions">
          查看全部
        </Link>
      </div>
      <div className="list">
        {items.length === 0 ? <div className="empty-state">还没有账单，先从“记一笔”开始。</div> : null}
        {items.map((row) => (
          <div className="transaction-row" key={row.id}>
            <div className="row-main">
              <span className={`icon-badge ${row.type}`} aria-hidden="true">
                {row.type === 'income' ? '收' : '支'}
              </span>
              <div className="row-title">
                <strong>{row.note || row.category?.name || '未命名账单'}</strong>
                <small>
                  {formatDateTime(row.occurredAt)} · {transactionTypeLabel(row.type)} · {row.category?.name || '-'} ·{' '}
                  {row.account?.name || '-'}
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
