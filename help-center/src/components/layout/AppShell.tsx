import { useState, useEffect, useCallback } from 'react'
import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import { SearchDialog } from '@/components/search/SearchDialog'

export function AppShell() {
  const [searchOpen, setSearchOpen] = useState(false)

  const openSearch = useCallback(() => setSearchOpen(true), [])
  const closeSearch = useCallback(() => setSearchOpen(false), [])

  // Global ⌘K / Ctrl+K shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setSearchOpen(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  return (
    <div className="min-h-screen" style={{ backgroundColor: 'var(--hc-bg)' }}>
      <TopBar onSearchClick={openSearch} />
      <Outlet />
      <SearchDialog open={searchOpen} onClose={closeSearch} />
    </div>
  )
}
