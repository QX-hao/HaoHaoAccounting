import Link from 'next/link';

const quickActions = [
  { href: '/transactions', icon: '+', title: '快速记账', desc: '收入支出' },
  { href: '/reports', icon: '↗', title: '统计分析', desc: '分类趋势' },
  { href: '/io', icon: '⇄', title: '导入导出', desc: 'CSV / Excel' },
  { href: '/categories', icon: '◇', title: '分类预算', desc: '管理标签' },
];

export function QuickActions() {
  return (
    <section className="quick-grid">
      {quickActions.map((item) => (
        <Link className="quick-action" href={item.href} key={item.href}>
          <span>{item.icon}</span>
          <strong>{item.title}</strong>
          <small>{item.desc}</small>
        </Link>
      ))}
    </section>
  );
}
