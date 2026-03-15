import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  getOrCreateAnonymousId,
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

  });

  describe('session persistence', () => {
    it('persistSession stores token in localStorage', () => {
      persistSession('key1', 'tok123', '2030-01-01T00:00:00Z');
      expect(localStorage.setItem).toHaveBeenCalledWith(
        'helpin_ws_key1',
        JSON.stringify({ session_token: 'tok123', expires_at: '2030-01-01T00:00:00Z' })
      );
    });

    it('getStoredSession returns stored session', () => {
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_key1') {
          return JSON.stringify({ session_token: 'tok123', expires_at: '2030-01-01T00:00:00Z' });
        }
        return null;
      });
      const session = getStoredSession('key1');
      expect(session).toEqual({ session_token: 'tok123', expires_at: '2030-01-01T00:00:00Z' });
    });

    it('getStoredSession returns null for expired session', () => {
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_ws_key1') {
          return JSON.stringify({ session_token: 'tok123', expires_at: '2020-01-01T00:00:00Z' });
        }
        return null;
      });
      const session = getStoredSession('key1');
      expect(session).toBeNull();
      expect(localStorage.removeItem).toHaveBeenCalledWith('helpin_ws_key1');
    });

    it('getStoredSession returns null for missing data', () => {
      (localStorage.getItem as any).mockReturnValue(null);
      expect(getStoredSession('key1')).toBeNull();
    });

    it('getStoredSession returns null for invalid JSON', () => {
      (localStorage.getItem as any).mockReturnValue('not-json');
      expect(getStoredSession('key1')).toBeNull();
    });

    it('clearSession removes from localStorage', () => {
      clearSession('key1');
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
      (localStorage.getItem as any).mockImplementation((key: string) => {
        if (key === 'helpin_wc_key1') {
          return JSON.stringify({ config: { color: 'red' }, cached_at: now - 1000 }); // 1s ago
        }
        return null;
      });
      const config = getCachedConfig('key1');
      expect(config).toEqual({ color: 'red' });
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
