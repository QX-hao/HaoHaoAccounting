import type { Category } from '@/lib/types';

export function CategoryGrid({ categories }: { categories: Category[] }) {
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
        </div>
      ))}
    </section>
  );
}
