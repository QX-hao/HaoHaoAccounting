import { useEffect, useState } from 'react';
import { clearToken, getToken, logout, setToken } from '../../shared/api/client';
import { login, verifyCurrentUser } from './api';

export function useSession() {
  const [ready, setReady] = useState(false);
  const [authed, setAuthed] = useState(false);
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    async function boot() {
      const token = await getToken();
      if (token) {
        try {
          await verifyCurrentUser();
          setAuthed(true);
        } catch {
          await clearToken();
          setAuthed(false);
        }
      }
      setReady(true);
    }
    boot();
  }, []);

  async function signIn() {
    const nextUsername = username.trim();
    const nextPassword = password.trim();
    if (!nextUsername) {
      setError('请输入用户名');
      return false;
    }
    if (!nextPassword) {
      setError('请输入密码');
      return false;
    }

    try {
      setError('');
      const resp = await login({ username: nextUsername, password: nextPassword });
      await setToken(resp.token);
      setAuthed(true);
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
      return false;
    }
  }

  async function signOut() {
    await logout();
    await clearToken();
    setAuthed(false);
  }

  return {
    ready,
    authed,
    username,
    password,
    error,
    setError,
    setUsername,
    setPassword,
    signIn,
    signOut,
  };
}
