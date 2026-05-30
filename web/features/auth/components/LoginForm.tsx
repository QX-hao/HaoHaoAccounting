'use client';

import type { FormEvent } from 'react';

type Props = {
  username: string;
  password: string;
  error: string;
  loading: boolean;
  onUsernameChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
};

export function LoginForm({ username, password, error, loading, onUsernameChange, onPasswordChange, onSubmit }: Props) {
  return (
    <section className="login-card">
      <div>
        <span className="eyebrow">Login</span>
        <h2>登录好好记账</h2>
      </div>

      <form className="grid" onSubmit={onSubmit}>
        <input autoComplete="username" placeholder="用户名" value={username} onChange={(e) => onUsernameChange(e.target.value)} />
        <input
          autoComplete="current-password"
          placeholder="密码"
          type="password"
          value={password}
          onChange={(e) => onPasswordChange(e.target.value)}
        />
        {error ? <div className="error">{error}</div> : null}
        <button className="primary" disabled={loading} type="submit">
          {loading ? '登录中...' : '进入账本'}
        </button>
      </form>
    </section>
  );
}
