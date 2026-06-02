import type { Account } from '@/lib/types';
import { formatMoney } from '@/lib/format';

const accountTypeLabel: Record<string, string> = {
  cash: '现金',
  bank: '银行卡',
  alipay: '支付宝',
  wechat: '微信',
  custom: '自定义',
};

type Props = {
  accounts: Account[];
  onEdit: (account: Account) => void;
  onDelete: (account: Account) => void;
  disabled?: boolean;
};

export function AccountGrid({ accounts, onEdit, onDelete, disabled = false }: Props) {
  return (
    <section className="grid three">
      {accounts.length === 0 ? <div className="empty-state">暂无账户。</div> : null}
      {accounts.map((item) => (
        <div className="card stat-card" key={item.id}>
          <span className="account-dot" aria-hidden="true">
            {item.name.slice(0, 1)}
          </span>
          <span className="label">{accountTypeLabel[item.type] || item.type}</span>
          <span className="value">{formatMoney(item.balance)}</span>
          <span className="hint">{item.name}</span>
          <div className="row-actions">
            <button className="ghost" type="button" disabled={disabled} onClick={() => onEdit(item)}>
              编辑
            </button>
            <button className="ghost danger" type="button" disabled={disabled} onClick={() => onDelete(item)}>
              删除
            </button>
          </div>
        </div>
      ))}
    </section>
  );
}
