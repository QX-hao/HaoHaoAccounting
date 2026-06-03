import type { Budget, Category } from '@/lib/types';
import { formatMoney } from '@/lib/format';

type Props = {
  budgets: Budget[];
  categories: Category[];
  month: string;
  categoryId: number;
  amount: string;
  editingId: number;
  onMonthChange: (value: string) => void;
  onCategoryChange: (value: number) => void;
  onAmountChange: (value: string) => void;
  onSave: () => void;
  onEdit: (budget: Budget) => void;
  onDelete: (id: number) => void;
  onCancelEdit: () => void;
};

export function BudgetManager({
  budgets,
  categories,
  month,
  categoryId,
  amount,
  editingId,
  onMonthChange,
  onCategoryChange,
  onAmountChange,
  onSave,
  onEdit,
  onDelete,
  onCancelEdit,
}: Props) {
  const expenseCategories = categories.filter((item) => item.type === 'expense');

  return (
    <section className="panel">
      <div className="hero-topline" style={{ marginBottom: 16 }}>
        <div>
          <span className="eyebrow">Plan</span>
          <h3>预算管理</h3>
        </div>
        {editingId ? (
          <button className="ghost" type="button" onClick={onCancelEdit}>
            取消编辑
          </button>
        ) : null}
      </div>

      <div className="form-grid" style={{ marginBottom: 16 }}>
        <input type="month" value={month} onChange={(e) => onMonthChange(e.target.value)} />
        <select value={categoryId} onChange={(e) => onCategoryChange(Number(e.target.value))}>
          <option value={0}>全部支出</option>
          {expenseCategories.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
        <input inputMode="decimal" placeholder="预算金额" value={amount} onChange={(e) => onAmountChange(e.target.value)} />
        <button className="primary" type="button" onClick={onSave}>
          {editingId ? '保存预算' : '新增预算'}
        </button>
      </div>

      <div className="list">
        {budgets.length ? null : <div className="empty-state">暂无预算。</div>}
        {budgets.map((budget) => (
          <div className="list-row actionable" key={budget.id}>
            <div className="row-title">
              <strong>{budgetLabel(budget, expenseCategories)}</strong>
              <small>{budget.month}</small>
            </div>
            <span className="amount">{formatMoney(budget.amount)}</span>
            <div className="row-actions">
              <button className="ghost" type="button" onClick={() => onEdit(budget)}>
                编辑
              </button>
              <button className="ghost danger" type="button" onClick={() => onDelete(budget.id)}>
                删除
              </button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function budgetLabel(budget: Budget, categories: Category[]) {
  if (!budget.categoryId) return '全部支出';
  return categories.find((item) => item.id === budget.categoryId)?.name || '未知分类';
}
