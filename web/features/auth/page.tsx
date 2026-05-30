'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { clearToken, getToken, setToken, setUser } from '@/lib/auth';
import { login, verifyCurrentUser } from './api';
import { LoginForm } from './components/LoginForm';
import { LoginHero } from './components/LoginHero';

export default function LoginFeaturePage() {
  const router = useRouter();
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let active = true;

    async function redirectAuthedUser() {
      if (!getToken()) return;

      try {
        await verifyCurrentUser();
        if (active) {
          router.replace('/overview');
        }
      } catch {
        clearToken();
      }
    }

    redirectAuthedUser();

    return () => {
      active = false;
    };
  }, [router]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');

    const nextUsername = username.trim();
    const nextPassword = password.trim();
    if (!nextUsername) {
      setError('请输入用户名');
      return;
    }
    if (!nextPassword) {
      setError('请输入密码');
      return;
    }

    setLoading(true);
    try {
      const resp = await login({ username: nextUsername, password: nextPassword });
      setToken(resp.token);
      setUser(resp.user);
      router.replace('/overview');
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="login-shell">
      <LoginHero />
      <LoginForm
        username={username}
        password={password}
        error={error}
        loading={loading}
        onUsernameChange={setUsername}
        onPasswordChange={setPassword}
        onSubmit={onSubmit}
      />
    </main>
  );
}
