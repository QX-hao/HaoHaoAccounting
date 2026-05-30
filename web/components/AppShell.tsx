'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { clearToken } from '@/lib/auth';

const navItems = [
  { href: '/overview', label: '首页', icon: '⌂' },
  { href: '/transactions', label: '记一笔', icon: '+' },
  { href: '/reports', label: '统计', icon: '⌁' },
  { href: '/io', label: '导入导出', icon: '⇄' },
  { href: '/categories', label: '分类', icon: '◇' },
  { href: '/accounts', label: '账户', icon: '▣' },
  { href: '/settings', label: '我的', icon: '○' },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  return (
    <div className="layout">
      <aside className="sidebar">
        <Link className="brand" href="/overview" aria-label="好好记账首页">
          <span className="brand-mark">好</span>
          <span>
            <strong>好好记账</strong>
            <small>三秒记一笔</small>
          </span>
        </Link>
        <nav aria-label="主导航">
          {navItems.map((item) => (
            <Link
              key={item.href}
              className={pathname === item.href ? 'active' : ''}
              href={item.href}
            >
              <span className="nav-icon" aria-hidden="true">
                {item.icon}
              </span>
              <span>{item.label}</span>
            </Link>
          ))}
        </nav>
        <div className="sidebar-panel">
          <span className="eyebrow">今日提醒</span>
          <strong>别忘了补记晚餐</strong>
          <p>保持账本连续，统计才会更准。</p>
        </div>
        <button
          className="logout"
          onClick={() => {
            clearToken();
            router.replace('/login');
          }}
        >
          退出登录
        </button>
      </aside>
      <main className="content">{children}</main>
    </div>
  );
}
