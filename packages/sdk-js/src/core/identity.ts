import { CookieManager } from '../utils/cookie';

const cookieManager = new CookieManager();

/**
 * Shared identity utilities used by both HelpinClient (analytics) and WidgetManager (chat).
 * The anonymous_id is the single durable browser identity that ties analytics events,
 * chat conversations, and CRM contacts together.
 */

// ─── Anonymous ID (cookie-based, 10-year TTL) ────────────────────────

/**
 * Gets or creates a durable anonymous ID for this browser.
 * Stored as a cookie so it survives localStorage clears.
 * Migrates from legacy keys on first call.
 */
export function getOrCreateAnonymousId(widgetKey: string): string {
  const cookieName = `helpin_aid_${widgetKey}`;
  const existing = cookieManager.get(cookieName);
  if (existing) return existing;

  // Check URL params for cross-domain linking (_hp parameter)
  let id: string | null = null;
  if (typeof window !== 'undefined') {
    const urlParams = new URLSearchParams(window.location.search);
    id = urlParams.get('_hp');
  }

  if (!id) {
    id = generateUUID();
  }

  const tenYearsInDays = 365 * 10;
  cookieManager.set(cookieName, id, tenYearsInDays, shouldUseSecureCookies());
  return id;
}

/** Clears the durable browser visitor ID. Used by shutdown on customer logout. */
export function clearAnonymousId(widgetKey: string): void {
  try {
    cookieManager.delete(`helpin_aid_${widgetKey}`);
  } catch {
    // ignore
  }
}

// ─── Widget Session Persistence (root-domain cookie) ────────────────

export interface StoredSession {
  session_token: string;
  expires_at: string; // ISO date string
}

/**
 * Reads a stored widget session from the shared root-domain cookie.
 * Legacy origin-scoped localStorage sessions are migrated on first read.
 */
export function getStoredSession(widgetKey: string): StoredSession | null {
  const cookieName = `helpin_session_${widgetKey}`;
  try {
    const raw = cookieManager.get(cookieName);
    if (raw) {
      const session = parseStoredSession(raw);
      if (!session) {
        cookieManager.delete(cookieName);
      } else {
        return session;
      }
    }
  } catch {
    cookieManager.delete(cookieName);
  }

  // Backwards-compatible migration from the old origin-scoped storage key.
  try {
    const legacyRaw = localStorage.getItem(`helpin_ws_${widgetKey}`);
    if (!legacyRaw) return null;
    const session = parseStoredSession(legacyRaw);
    if (!session) {
      clearSession(widgetKey);
      return null;
    }

    persistSession(widgetKey, session.session_token, session.expires_at);
    return session;
  } catch {
    return null;
  }
}

/**
 * Persists a widget session token in a root-domain cookie so the same active
 * conversation can be restored across sibling subdomains.
 */
export function persistSession(widgetKey: string, sessionToken: string, expiresAt: string): void {
  const expiry = new Date(expiresAt);
  const remainingMs = expiry.getTime() - Date.now();
  if (!sessionToken || !Number.isFinite(expiry.getTime()) || remainingMs <= 0) {
    clearSession(widgetKey);
    return;
  }

  const cookieName = `helpin_session_${widgetKey}`;
  const serialized = JSON.stringify({ session_token: sessionToken, expires_at: expiresAt });
  let cookiePersisted = false;
  try {
    cookieManager.set(
      cookieName,
      serialized,
      remainingMs / (24 * 60 * 60 * 1000),
      typeof window !== 'undefined' && window.location.protocol === 'https:',
    );
    cookiePersisted = cookieManager.get(cookieName) === serialized;
  } catch {
    // Cookies may be unavailable due to browser policy.
  }

  try {
    if (cookiePersisted) {
      localStorage.removeItem(`helpin_ws_${widgetKey}`);
    } else {
      // Preserve same-origin continuity when the browser rejects cookies.
      localStorage.setItem(`helpin_ws_${widgetKey}`, serialized);
    }
  } catch {
    // Ignore localStorage persistence and cleanup failures.
  }
}

/**
 * Clears both the current session cookie and the legacy localStorage key.
 */
export function clearSession(widgetKey: string): void {
  try {
    cookieManager.delete(`helpin_session_${widgetKey}`);
  } catch {
    // ignore
  }
  try {
    localStorage.removeItem(`helpin_ws_${widgetKey}`);
  } catch {
    // ignore
  }
}

function parseStoredSession(raw: string): StoredSession | null {
  const session: unknown = JSON.parse(raw);
  if (!session || typeof session !== 'object') return null;
  const candidate = session as Partial<StoredSession>;
  if (typeof candidate.session_token !== 'string' || typeof candidate.expires_at !== 'string') return null;
  const expiresAt = new Date(candidate.expires_at);
  if (!Number.isFinite(expiresAt.getTime()) || expiresAt <= new Date()) return null;
  return candidate as StoredSession;
}

// ─── Identified User Persistence (cookie-based, cross-subdomain) ────

export interface StoredIdentity {
  email: string;
  firstName?: string;
  lastName?: string;
  name?: string; // legacy compatibility for older cookies
}

const IDENTITY_COOKIE_TTL_DAYS = 365;

/**
 * Persists the identified user's email and name as a cookie on the root domain.
 * Cookie-based (not localStorage) so it works across subdomains
 * (e.g. app.example.com and example.com share the same identity).
 */
export function persistIdentity(widgetKey: string, email: string, _name: string, firstName: string = '', lastName: string = ''): void {
  try {
    cookieManager.set(
      `helpin_uid_${widgetKey}`,
      JSON.stringify({ email, firstName: firstName.trim(), lastName: lastName.trim() }),
      IDENTITY_COOKIE_TTL_DAYS,
      shouldUseSecureCookies(),
    );
  } catch {
    // cookie may be unavailable
  }
}

/**
 * Reads a stored identified user from the identity cookie.
 * Returns null if not found or missing email.
 */
export function getStoredIdentity(widgetKey: string): StoredIdentity | null {
  try {
    // cookieManager.get() already decodes the value
    const raw = cookieManager.get(`helpin_uid_${widgetKey}`);
    if (!raw) return null;
    const identity: StoredIdentity = JSON.parse(raw);
    if (!identity.email) return null;
    return identity;
  } catch {
    return null;
  }
}

/**
 * Clears the stored identified user cookie.
 */
export function clearIdentity(widgetKey: string): void {
  try {
    cookieManager.delete(`helpin_uid_${widgetKey}`);
  } catch {
    // ignore
  }
}

// ─── Widget Config Cache ─────────────────────────────────────────────

/**
 * Clears the cached widget config from localStorage.
 */
export function clearConfigCache(widgetKey: string): void {
  try {
    localStorage.removeItem(`helpin_wc_${widgetKey}`);
  } catch {
    // ignore
  }
}

/**
 * Gets cached widget config from localStorage (10-minute TTL).
 */
export function getCachedConfig(widgetKey: string): any | null {
  try {
    const raw = localStorage.getItem(`helpin_wc_${widgetKey}`);
    if (!raw) return null;
    const cached = JSON.parse(raw);
    if (!cached.config || !cached.cached_at) return null;
    // Reject stale flat-format config (pre-nested branding migration)
    if (!cached.config.branding) {
      clearConfigCache(widgetKey);
      return null;
    }
    // 10-minute TTL
    if (Date.now() - cached.cached_at > 10 * 60 * 1000) {
      clearConfigCache(widgetKey);
      return null;
    }
    return cached.config;
  } catch {
    return null;
  }
}

/**
 * Caches widget config to localStorage with timestamp.
 */
export function cacheConfig(widgetKey: string, config: any): void {
  try {
    localStorage.setItem(
      `helpin_wc_${widgetKey}`,
      JSON.stringify({ config, cached_at: Date.now() })
    );
  } catch {
    // ignore
  }
}

// ─── Helpers ─────────────────────────────────────────────────────────

function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  // Fallback for older browsers
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

function shouldUseSecureCookies(): boolean {
  return typeof window !== 'undefined' && window.location.protocol === 'https:';
}
