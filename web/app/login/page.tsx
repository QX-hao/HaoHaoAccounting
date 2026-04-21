'use client';

import { FormEvent, useState } from 'react';
import { useRouter } from 'next/navigation';
import { request } from '@/lib/api';
import { setToken, setUser } from '@/lib/auth';

export default function LoginPage() {
  const router = useRouter();
  const [loginType, setLoginType] = useState<'phone' | 'email' | 'wechat'>('phone');
  const [identifier, setIdentifier] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const resp = await request<{ token: string; user: unknown }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ loginType, identifier, name }),
      });
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
    <div className="loading" style={{ padding: 24 }}>
      <form className="card" style={{ width: '100%', maxWidth: 420 }} onSubmit={onSubmit}>
        <h1 style={{ marginTop: 0 }}>登录好好记账</h1>
        <p className="muted">支持手机号、邮箱和微信登录（MVP 为免密模式）。</p>

        <div className="grid">
          <select value={loginType} onChange={(e) => setLoginType(e.target.value as 'phone' | 'email' | 'wechat')}>
            <option value="phone">手机号</option>
            <option value="email">邮箱</option>
            <option value="wechat">微信</option>
          </select>
          <input
            placeholder={loginType === 'phone' ? '请输入手机号' : loginType === 'email' ? '请输入邮箱' : '请输入微信标识'}
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
          />
          <input placeholder="昵称（可选）" value={name} onChange={(e) => setName(e.target.value)} />
          {error ? <div className="error">{error}</div> : null}
          <button className="primary" disabled={loading} type="submit">
            {loading ? '登录中...' : '登录'}
          </button>
        </div>
      </form>
    </div>
  );
}
