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

      try {
        // 只信任服务端 /me 校验结果；本地 token 存在不代表仍未过期或未被撤销。
        await request('/me');
        if (active) {
          setReady(true);
        }
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
