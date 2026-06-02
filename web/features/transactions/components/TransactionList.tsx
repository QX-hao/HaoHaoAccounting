import type { Transaction } from '@/lib/types';
import { formatDateTime, formatMoney } from '@/lib/format';

type Props = {
  transactions: Transaction[];
  total: number;
  page: number;
  pageSize: number;
  disabled?: boolean;
  onEdit: (transaction: Transaction) => void;
  onDelete: (transaction: Transaction) => void;
  onPageChange: (page: number) => void;
};

export function TransactionList({ transactions, total, page, pageSize, disabled = false, onEdit, onDelete, onPageChange }: Props) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 12 }}>
        <div>
          <span className="eyebrow">Ledger</span>
          <h3>账单列表</h3>
        </div>
        <span className="badge">共 {total} 条</span>
      </div>
      <div className="list">
        {transactions.length === 0 ? <div className="empty-state">暂无账单。</div> : null}
        {transactions.map((row) => (
          <div className="transaction-row actionable" key={row.id}>
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
            <div className="row-actions">
              <button className="ghost" type="button" disabled={disabled} onClick={() => onEdit(row)}>
                编辑
              </button>
              <button className="ghost danger" type="button" disabled={disabled} onClick={() => onDelete(row)}>
                删除
              </button>
            </div>
          </div>
        ))}
      </div>
      <div className="pagination">
        <button className="ghost" type="button" disabled={disabled || page <= 1} onClick={() => onPageChange(page - 1)}>
          上一页
        </button>
        <span className="badge">
          {page} / {totalPages}
        </span>
        <button className="ghost" type="button" disabled={disabled || page >= totalPages} onClick={() => onPageChange(page + 1)}>
          下一页
        </button>
      </div>
    </section>
  );
}
