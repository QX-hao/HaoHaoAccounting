'use client';

import { useEffect, useState } from 'react';
import { PageFrame } from '@/components/PageFrame';
import type { CurrentUser } from '@/lib/types';
import { getCurrentUser } from './api';
import { ProfileDetails } from './components/ProfileDetails';
import { ProfileHero } from './components/ProfileHero';

export default function SettingsFeaturePage() {
  const [me, setMe] = useState<CurrentUser | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      try {
        setMe(await getCurrentUser());
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      }
    }
    load();
  }, []);

  return (
    <PageFrame title="我的" subtitle="登录信息与基础偏好。">
      {error ? <div className="error">{error}</div> : null}
      <div className="grid two">
        <ProfileHero me={me} />
        <ProfileDetails me={me} />
      </div>
    </PageFrame>
  );
}
