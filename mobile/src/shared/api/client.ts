import AsyncStorage from '@react-native-async-storage/async-storage';

const TOKEN_KEY = 'haohao_token';
export const API_BASE = process.env.EXPO_PUBLIC_API_BASE || 'http://127.0.0.1:8080/api/v1';

export async function getToken() {
  return (await AsyncStorage.getItem(TOKEN_KEY)) || '';
}

export async function setToken(token: string) {
  await AsyncStorage.setItem(TOKEN_KEY, token);
}

export async function clearToken() {
  await AsyncStorage.removeItem(TOKEN_KEY);
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {});
  const token = await getToken();

  if (!(init.body instanceof FormData)) {
    headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  });

  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(data.error || '请求失败');
  }
  return data as T;
}
