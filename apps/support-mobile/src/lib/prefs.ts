import { LazyStore } from '@tauri-apps/plugin-store'
import { isTauri } from '@mobile/lib/host'

const PREFS_STORE_PATH = 'prefs/workspace.json'
const LAST_WORKSPACE_KEY = 'last_workspace_slug'

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
