'use client';

import { useEffect, useState } from 'react';
import { request } from '@/lib/api';
import { PageFrame } from '@/components/PageFrame';

type Me = {
  id: number;
  name: string;
  phone?: string;
  email?: string;
  wechatId?: string;
};

export default function SettingsPage() {
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        const user = await request<Me>('/me');
        setMe(user);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame title="个人设置" subtitle="登录信息与基础偏好">
      {error ? <div className="error">{error}</div> : null}
      <div className="card grid">
        <div>
          <strong>用户ID：</strong> {me?.id || '-'}
        </div>
        <div>
          <strong>昵称：</strong> {me?.name || '-'}
        </div>
        <div>
          <strong>手机号：</strong> {me?.phone || '-'}
        </div>
        <div>
          <strong>邮箱：</strong> {me?.email || '-'}
        </div>
        <div>
          <strong>微信标识：</strong> {me?.wechatId || '-'}
        </div>
      </div>
    </PageFrame>
  );
}
