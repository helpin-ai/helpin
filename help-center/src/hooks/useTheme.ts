import { useState, useEffect, useCallback } from 'react'
import type { HelpcenterThemeMode } from '@/lib/types'

type Theme = 'light' | 'dark'

function getStoredTheme(): Theme {
  if (typeof window === 'undefined') return 'light'
  const stored = localStorage.getItem('hc-theme')
  if (stored === 'dark' || stored === 'light') return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

export function useTheme(configThemeMode?: HelpcenterThemeMode) {
  const isForced = configThemeMode === 'light' || configThemeMode === 'dark'

  const [theme, setThemeState] = useState<Theme>(() => {
    if (isForced) return configThemeMode as Theme
    // Match the server render. The inline head script applies the stored theme
    // before paint, then the effect below synchronizes React state after hydration.
    return 'light'
  })

  useEffect(() => {
    if (isForced) {
      setThemeState(configThemeMode as Theme)
      return
    }
    setThemeState(getStoredTheme())
  }, [isForced, configThemeMode])

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  const setTheme = useCallback((t: Theme) => {
    if (isForced) return
    if (typeof window !== 'undefined') {
      localStorage.setItem('hc-theme', t)
    }
    setThemeState(t)
  }, [isForced])

  const toggleTheme = useCallback(() => {
    setTheme(theme === 'light' ? 'dark' : 'light')
  }, [theme, setTheme])

  return { theme, setTheme, toggleTheme, canToggle: !isForced }
}
