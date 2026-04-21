'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { clearToken } from '@/lib/auth';

const navItems = [
  { href: '/overview', label: '总览' },
  { href: '/transactions', label: '账单明细' },
  { href: '/reports', label: '报表分析' },
  { href: '/io', label: '导入导出' },
  { href: '/categories', label: '分类管理' },
  { href: '/accounts', label: '账户管理' },
  { href: '/settings', label: '个人设置' },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  return (
    <div className="layout">
      <aside className="sidebar">
        <h1>好好记账</h1>
        <nav>
          {navItems.map((item) => (
            <Link
              key={item.href}
              className={pathname === item.href ? 'active' : ''}
              href={item.href}
            >
              {item.label}
            </Link>
          ))}
        </nav>
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
