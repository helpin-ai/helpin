import { useState, useEffect, useCallback } from 'react'
import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import { Footer } from './Footer'
import { SearchDialog } from '@/components/search/SearchDialog'

export function AppShell() {
  const [searchOpen, setSearchOpen] = useState(false)

  const openSearch = useCallback(() => setSearchOpen(true), [])
  const closeSearch = useCallback(() => setSearchOpen(false), [])

  // Global Cmd+K / Ctrl+K shortcut
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

  // Allow child components (e.g. homepage) to open search via custom event
  useEffect(() => {
    const handler = () => setSearchOpen(true)
    window.addEventListener('open-help-search', handler)
    return () => window.removeEventListener('open-help-search', handler)
  }, [])

  return (
    <div className="min-h-screen flex flex-col">
      <TopBar onSearchClick={openSearch} />
      <div className="flex-1">
        <Outlet />
      </div>
      <Footer />
      <SearchDialog open={searchOpen} onClose={closeSearch} />
    </div>
  )
}
