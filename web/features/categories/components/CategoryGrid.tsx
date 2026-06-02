import type { Category } from '@/lib/types';

type Props = {
  categories: Category[];
  onEdit: (category: Category) => void;
  onDelete: (category: Category) => void;
  disabled?: boolean;
};

export function CategoryGrid({ categories, onEdit, onDelete, disabled = false }: Props) {
  return (
    <section className="grid three">
      {categories.length === 0 ? <div className="empty-state">暂无分类。</div> : null}
      {categories.map((item) => (
        <div className="card stat-card" key={item.id}>
          <span className="category-dot" aria-hidden="true">
            {item.name.slice(0, 1)}
          </span>
          <span className={`pill ${item.type}`}>{item.type === 'income' ? '收入' : '支出'}</span>
          <span className="value" style={{ fontSize: 24 }}>
            {item.name}
          </span>
          <span className="hint">{item.isSystem ? '系统分类' : '自定义分类'}</span>
          <div className="row-actions">
            <button className="ghost" type="button" disabled={disabled || item.isSystem} onClick={() => onEdit(item)}>
              编辑
            </button>
            <button className="ghost danger" type="button" disabled={disabled || item.isSystem} onClick={() => onDelete(item)}>
              删除
            </button>
          </div>
        </div>
      ))}
    </section>
  );
}
