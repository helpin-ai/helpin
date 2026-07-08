import { LazyStore } from '@tauri-apps/plugin-store'
import { isTauri } from '@mobile/lib/host'

const PREFS_STORE_PATH = 'prefs/workspace.json'
const LAST_WORKSPACE_KEY = 'last_workspace_slug'
const PUSH_PRIMING_KEY = 'push_priming'

/** How long after a "Not now" decline before the priming sheet may reappear. */
const PUSH_PRIMING_RE_ASK_AFTER_MS = 7 * 24 * 60 * 60 * 1000

let store: LazyStore | null = null

function getStore(): LazyStore {
  if (!store) store = new LazyStore(PREFS_STORE_PATH)
  return store
}

/** Last workspace the user picked, or `null` in browser preview / on first run. */
export async function getLastWorkspaceSlug(): Promise<string | null> {
  if (!isTauri()) return null
  try {
    const value = await getStore().get<string>(LAST_WORKSPACE_KEY)
    return typeof value === 'string' ? value : null
  } catch {
    return null
  }
}

export async function setLastWorkspaceSlug(slug: string): Promise<void> {
  if (!isTauri()) return
  try {
    await getStore().set(LAST_WORKSPACE_KEY, slug)
    await getStore().save()
  } catch {
    // Best-effort — losing the "remember my workspace" preference isn't
    // worth surfacing an error over.
  }
}

/**
 * Persisted decision from the push-notification permission priming sheet
 * (`src/push/permission-priming-sheet.tsx`).
 *
 * - `'later'`: user tapped "Not now" — re-ask no sooner than
 *   {@link PUSH_PRIMING_RE_ASK_AFTER_MS} after `at`.
 * - `'enabled'`: `registerForPush()` succeeded — never show the sheet again.
 *
 * No persisted pref at all (`null`) means never-asked.
 */
export interface PushPrimingPref {
  decision: 'later' | 'enabled'
  at: string
}

/** Last priming sheet decision, or `null` in browser preview / never asked. */
export async function getPushPrimingPref(): Promise<PushPrimingPref | null> {
  if (!isTauri()) return null
  try {
    const value = await getStore().get<PushPrimingPref>(PUSH_PRIMING_KEY)
    return value ?? null
  } catch {
    return null
  }
}

export async function setPushPrimingPref(pref: PushPrimingPref): Promise<void> {
  if (!isTauri()) return
  try {
    await getStore().set(PUSH_PRIMING_KEY, pref)
    await getStore().save()
  } catch {
    // Best-effort — losing the priming decision just means the sheet might
    // show again sooner than intended; not worth surfacing an error over.
  }
}

/**
 * PURE decision function for whether the permission priming sheet should be
 * shown: never-asked (`null`) always shows; `'enabled'` never shows again;
 * `'later'` re-asks once {@link PUSH_PRIMING_RE_ASK_AFTER_MS} has elapsed
 * since the decline.
 */
export function shouldShowPriming(pref: PushPrimingPref | null, now: Date): boolean {
  if (!pref) return true
  if (pref.decision === 'enabled') return false
  const elapsed = now.getTime() - new Date(pref.at).getTime()
  return elapsed >= PUSH_PRIMING_RE_ASK_AFTER_MS
}
