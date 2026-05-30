'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { request } from '@/lib/api';
import { clearToken, getToken } from '@/lib/auth';

export function AuthGate({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let active = true;

    async function verifyToken() {
      const token = getToken();
      if (!token) {
        router.replace('/login');
        return;
      }

      setReady(true);

      try {
        await request('/me');
      } catch {
        clearToken();
        if (active) {
          router.replace('/login');
        }
      }
    }

    verifyToken();

    return () => {
      active = false;
    };
  }, [router]);

  if (!ready) {
    return <div className="loading">加载中...</div>;
  }
  return <>{children}</>;
}
