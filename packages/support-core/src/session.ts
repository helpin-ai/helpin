export interface SessionSnapshot {
  accessToken: string
  refreshToken: string
  rememberMe?: boolean
}

type MaybePromise<T> = T | Promise<T>

export interface SessionStorageAdapter {
  getAccessToken(): string | null
  getRefreshToken(): string | null
  getRememberMe(): boolean
  hydrate?(): MaybePromise<void>
  setSession(snapshot: SessionSnapshot): MaybePromise<void>
  clearSession(): MaybePromise<void>
}

const ACCESS_TOKEN_KEY = 'access_token'
const REFRESH_TOKEN_KEY = 'refresh_token'
const REMEMBER_ME_KEY = 'remember_me'

interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

function createStorageAdapter(storage?: StorageLike): SessionStorageAdapter {
  return {
    getAccessToken: () => storage?.getItem(ACCESS_TOKEN_KEY) ?? null,
    getRefreshToken: () => storage?.getItem(REFRESH_TOKEN_KEY) ?? null,
    getRememberMe: () => (storage?.getItem(REMEMBER_ME_KEY) ?? '0') === '1',
    setSession: ({ accessToken, refreshToken, rememberMe }) => {
      storage?.setItem(ACCESS_TOKEN_KEY, accessToken)
      storage?.setItem(REFRESH_TOKEN_KEY, refreshToken)
      storage?.setItem(REMEMBER_ME_KEY, rememberMe ? '1' : '0')
    },
    clearSession: () => {
      storage?.removeItem(ACCESS_TOKEN_KEY)
      storage?.removeItem(REFRESH_TOKEN_KEY)
      storage?.removeItem(REMEMBER_ME_KEY)
    },
  }
}

export function createBrowserSessionStorage(storage?: StorageLike): SessionStorageAdapter {
  if (storage) {
    return createStorageAdapter(storage)
  }

  if (typeof window === 'undefined' || !window.localStorage) {
    return createMemorySessionStorage()
  }

  return createStorageAdapter(window.localStorage)
}

export function createMemorySessionStorage(initial?: Partial<SessionSnapshot>): SessionStorageAdapter {
  let accessToken = initial?.accessToken ?? null
  let refreshToken = initial?.refreshToken ?? null
  let rememberMe = initial?.rememberMe ?? false

  return {
    getAccessToken: () => accessToken,
    getRefreshToken: () => refreshToken,
    getRememberMe: () => rememberMe,
    setSession: (snapshot) => {
      accessToken = snapshot.accessToken
      refreshToken = snapshot.refreshToken
      rememberMe = snapshot.rememberMe ?? false
    },
    clearSession: () => {
      accessToken = null
      refreshToken = null
      rememberMe = false
    },
  }
}

let activeSessionStorage: SessionStorageAdapter =
  typeof window === 'undefined' ? createMemorySessionStorage() : createBrowserSessionStorage()

export function configureSessionStorage(storage: SessionStorageAdapter): void {
  activeSessionStorage = storage
}

export function getSessionStorage(): SessionStorageAdapter {
  return activeSessionStorage
}

export function getAccessToken(): string | null {
  return activeSessionStorage.getAccessToken()
}

export function getRefreshToken(): string | null {
  return activeSessionStorage.getRefreshToken()
}

export function getRememberMe(): boolean {
  return activeSessionStorage.getRememberMe()
}

export async function hydrateSessionStorage(): Promise<void> {
  await activeSessionStorage.hydrate?.()
}

export function readSession(): SessionSnapshot | null {
  const accessToken = getAccessToken()
  const refreshToken = getRefreshToken()

  if (!accessToken || !refreshToken) {
    return null
  }

  return {
    accessToken,
    refreshToken,
    rememberMe: getRememberMe(),
  }
}

export async function writeSession(snapshot: SessionSnapshot): Promise<void> {
  await activeSessionStorage.setSession(snapshot)
}

export async function clearSession(): Promise<void> {
  await activeSessionStorage.clearSession()
}
