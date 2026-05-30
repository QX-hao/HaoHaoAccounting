'use client';

import { AuthGate } from './AuthGate';
import { AppShell } from './AppShell';

export function PageFrame({
  title,
  subtitle,
  action,
  children,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <AuthGate>
      <AppShell>
        <div className="page-stack">
          <div className="page-heading">
            <div>
              <span className="eyebrow">HaoHao Ledger</span>
              <h2>{title}</h2>
              {subtitle ? <p>{subtitle}</p> : null}
            </div>
            {action ? <div className="page-action">{action}</div> : null}
          </div>
          {children}
        </div>
      </AppShell>
    </AuthGate>
  );
}
