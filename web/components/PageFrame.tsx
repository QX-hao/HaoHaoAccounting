'use client';

import { AuthGate } from './AuthGate';
import { AppShell } from './AppShell';

export function PageFrame({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <AuthGate>
      <AppShell>
        <div className="grid" style={{ gap: 12 }}>
          <div>
            <h2 style={{ margin: 0 }}>{title}</h2>
            {subtitle ? <p className="muted">{subtitle}</p> : null}
          </div>
          {children}
        </div>
      </AppShell>
    </AuthGate>
  );
}
