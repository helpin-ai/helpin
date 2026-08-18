import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  getOrCreateAnonymousId,
  clearAnonymousId,
  getStoredSession,
  persistSession,
  clearSession,
  clearConfigCache,
  getCachedConfig,
  cacheConfig,
} from '../../../src/core/identity';

// Mock CookieManager
const mockCookies: Record<string, string> = {};
vi.mock('../../../src/utils/cookie', () => ({
  CookieManager: vi.fn().mockImplementation(() => ({
    get: (name: string) => mockCookies[name] || null,
    set: (name: string, value: string) => { mockCookies[name] = value; },
    delete: (name: string) => { delete mockCookies[name]; },
  })),
}));

describe('identity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.keys(mockCookies).forEach((k) => delete mockCookies[k]);
    (localStorage.getItem as any).mockReturnValue(null);
    (localStorage.setItem as any).mockClear();
    (localStorage.removeItem as any).mockClear();
  });

  describe('getOrCreateAnonymousId', () => {
    it('creates a new anonymous ID when none exists', () => {
      const id = getOrCreateAnonymousId('test-key');
      expect(id).toBeTruthy();
      expect(id).toMatch(/^[0-9a-f-]{36}$/); // UUID format
      expect(mockCookies['helpin_aid_test-key']).toBe(id);
    });

    it('returns existing anonymous ID from cookie', () => {
      mockCookies['helpin_aid_test-key'] = 'existing-uuid';
      const id = getOrCreateAnonymousId('test-key');
      expect(id).toBe('existing-uuid');
    });

    it('clears the anonymous ID on logout', () => {
      mockCookies['helpin_aid_test-key'] = 'existing-uuid';

      clearAnonymousId('test-key');

      expect(mockCookies['helpin_aid_test-key']).toBeUndefined();
    });

  });

  describe('session persistence', () => {
    it('persistSession stores the token in a shared cookie and clears legacy storage', () => {
      persistSession('key1', 'tok123', '2030-01-01T00:00:00Z');
      expect(mockCookies['helpin_session_key1']).toBe(
        JSON.stringify({ session_token: 'tok123', expires_at: '2030-01-01T00:00:00Z' }),
      );
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_key1');
    });

    it('getStoredSession returns the cookie session before legacy storage', () => {
      mockCookies['helpin_session_key1'] = JSON.stringify({
        session_token: 'cookie-token',
        expires_at: '2030-01-01T00:00:00Z',
      });
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_key1') {
          return JSON.stringify({ session_token: 'legacy-token', expires_at: '2030-01-01T00:00:00Z' });
        }
        return null;
      });
      const session = getStoredSession('key1');
      expect(session).toEqual({ session_token: 'cookie-token', expires_at: '2030-01-01T00:00:00Z' });
      expect(localStorage.getItem).not.toHaveBeenCalled();
    });

    it('migrates a legacy localStorage session into the cookie', () => {
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_key1') {
          return JSON.stringify({ session_token: 'legacy-token', expires_at: '2030-01-01T00:00:00Z' });
        }
        return null;
      });

      const session = getStoredSession('key1');
      expect(session).toEqual({ session_token: 'legacy-token', expires_at: '2030-01-01T00:00:00Z' });
      expect(mockCookies['helpin_session_key1']).toBe(
        JSON.stringify({ session_token: 'legacy-token', expires_at: '2030-01-01T00:00:00Z' }),
      );
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_key1');
    });

    it('getStoredSession clears an expired cookie session', () => {
      mockCookies['helpin_session_key1'] = JSON.stringify({
        session_token: 'tok123',
        expires_at: '2020-01-01T00:00:00Z',
      });
      const session = getStoredSession('key1');
      expect(session).toBeNull();
      expect(mockCookies['helpin_session_key1']).toBeUndefined();
    });

    it('getStoredSession returns null for missing data', () => {
      (localStorage.getItem as any).mockReturnValue(null);
      expect(getStoredSession('key1')).toBeNull();
    });

    it('getStoredSession returns null for invalid JSON', () => {
      (localStorage.getItem as any).mockReturnValue('not-json');
      expect(getStoredSession('key1')).toBeNull();
    });

    it('clearSession removes the cookie and legacy storage', () => {
      mockCookies['helpin_session_key1'] = 'stored';
      clearSession('key1');
      expect(mockCookies['helpin_session_key1']).toBeUndefined();
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_key1');
    });
  });

  describe('config cache', () => {
    it('cacheConfig stores config with timestamp', () => {
      const config = { brand_color: '#fff' };
      cacheConfig('key1', config);
      expect(localStorage.setItem).toHaveBeenCalled();
      const call = (localStorage.setItem as any).mock.calls[0];
      expect(call[0]).toBe('helpin_wc_key1');
      const stored = JSON.parse(call[1]);
      expect(stored.config).toEqual(config);
      expect(stored.cached_at).toBeTypeOf('number');
    });

    it('getCachedConfig returns config within TTL', () => {
      const now = Date.now();
      const mockConfig = { branding: { primaryColor: '#000' }, features: {} };
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_wc_key1') {
          return JSON.stringify({ config: mockConfig, cached_at: now - 1000 }); // 1s ago
        }
        return null;
      });
      const config = getCachedConfig('key1');
      expect(config).toEqual(mockConfig);
    });

    it('getCachedConfig returns null for expired cache', () => {
      const expired = Date.now() - 11 * 60 * 1000; // 11 minutes ago
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_wc_key1') {
          return JSON.stringify({ config: { color: 'red' }, cached_at: expired });
        }
        return null;
      });
      const config = getCachedConfig('key1');
      expect(config).toBeNull();
    });

    it('clearConfigCache removes from localStorage', () => {
      clearConfigCache('key1');
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_wc_key1');
    });
  });
});
