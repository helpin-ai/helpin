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
  cookieManager.set(cookieName, id, tenYearsInDays);
  return id;
}

// ─── Widget Session Persistence (localStorage) ──────────────────────

export interface StoredSession {
  session_token: string;
  expires_at: string; // ISO date string
}

/**
 * Reads a stored widget session from localStorage.
 * Returns null if not found or expired.
 */
export function getStoredSession(widgetKey: string): StoredSession | null {
  try {
    const raw = localStorage.getItem(`helpin_ws_${widgetKey}`);
    if (!raw) return null;
    const session: StoredSession = JSON.parse(raw);
    if (!session.session_token || !session.expires_at) return null;
    // Client-side expiry check
    if (new Date(session.expires_at) <= new Date()) {
      clearSession(widgetKey);
      return null;
    }
    return session;
  } catch {
    return null;
  }
}

/**
 * Persists a widget session token to localStorage.
 */
export function persistSession(widgetKey: string, sessionToken: string, expiresAt: string): void {
  try {
    localStorage.setItem(
      `helpin_ws_${widgetKey}`,
      JSON.stringify({ session_token: sessionToken, expires_at: expiresAt })
    );
  } catch {
    // localStorage may be full or unavailable
  }
}

/**
 * Clears the stored widget session from localStorage.
 */
export function clearSession(widgetKey: string): void {
  try {
    localStorage.removeItem(`helpin_ws_${widgetKey}`);
  } catch {
    // ignore
  }
}

// ─── Identified User Persistence (cookie-based, cross-subdomain) ────

export interface StoredIdentity {
  email: string;
  name: string;
}

const IDENTITY_COOKIE_TTL_DAYS = 365;

/**
 * Persists the identified user's email and name as a cookie on the root domain.
 * Cookie-based (not localStorage) so it works across subdomains
 * (e.g. app.example.com and example.com share the same identity).
 */
export function persistIdentity(widgetKey: string, email: string, name: string): void {
  try {
    cookieManager.set(`helpin_uid_${widgetKey}`, JSON.stringify({ email, name }), IDENTITY_COOKIE_TTL_DAYS);
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
