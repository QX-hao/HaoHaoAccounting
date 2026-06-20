const DEFAULT_API_BASE = 'http://localhost:8080/api/v1';

export const API_BASE = normalizePublicApiBase(
  process.env.NEXT_PUBLIC_API_BASE,
  DEFAULT_API_BASE,
  'NEXT_PUBLIC_API_BASE',
);

function normalizePublicApiBase(value: string | undefined, fallback: string, name: string) {
  const raw = value?.trim() || fallback;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${name} must be an absolute http(s) URL`);
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error(`${name} must use http or https`);
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new Error(`${name} must not include credentials, query strings, or fragments`);
  }
  return url.toString().replace(/\/+$/, '');
}
