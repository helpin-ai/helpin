import { API_BASE } from '@/lib/api';

export function buildGoogleAuthStartUrl(redirect?: string | null): string {
  const url = new URL(`${API_BASE}/auth/google/start`);
  if (redirect) {
    url.searchParams.set('redirect', redirect);
  }
  return url.toString();
}
