import {
  createMemorySessionStorage,
  type SessionSnapshot,
  type SessionStorageAdapter,
} from '@helpin-ai/support-core'
import { LazyStore } from '@tauri-apps/plugin-store'

const SESSION_KEY = 'support_session'
const SESSION_STORE_PATH = 'auth/session.json'

function isSessionSnapshot(value: unknown): value is SessionSnapshot {
  if (!value || typeof value !== 'object') {
    return false
  }

  const snapshot = value as Partial<SessionSnapshot>
  return typeof snapshot.accessToken === 'string' && typeof snapshot.refreshToken === 'string'
}

export function createTauriSessionStorage(): SessionStorageAdapter {
  const memory = createMemorySessionStorage()
  const store = new LazyStore(SESSION_STORE_PATH)

  return {
    getAccessToken: () => memory.getAccessToken(),
    getRefreshToken: () => memory.getRefreshToken(),
    getRememberMe: () => memory.getRememberMe(),
    hydrate: async () => {
      const snapshot = await store.get<SessionSnapshot | null>(SESSION_KEY)
      if (isSessionSnapshot(snapshot)) {
        memory.setSession(snapshot)
        return
      }

      memory.clearSession()
    },
    setSession: async (snapshot) => {
      memory.setSession(snapshot)
      await store.set(SESSION_KEY, snapshot)
      await store.save()
    },
    clearSession: async () => {
      memory.clearSession()
      await store.delete(SESSION_KEY)
      await store.save()
    },
  }
}
