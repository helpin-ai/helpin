import { lazy, Suspense, useState, useEffect, useCallback } from 'react'
import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import { useDocsContext } from '@/contexts/DocsContext'

const SearchDialog = lazy(async () => {
  const module = await import('@/components/search/SearchDialog')
  return { default: module.SearchDialog }
})

const WIDGET_SCRIPT_ID = 'helpin-widget'
const WIDGET_SCRIPT_SRC = import.meta.env.VITE_WIDGET_SCRIPT_URL || 'https://cdn.helpin.ai/lib.js'
const WIDGET_HOST = import.meta.env.VITE_WIDGET_HOST || 'https://client.helpin.ai'

function HelpCenterChatWidget() {
  const { config } = useDocsContext()

  useEffect(() => {
    const widgetKey = config.support_widget_key?.trim()
    if (!config.is_published || config.chat_widget_enabled === false || !widgetKey) return

    const existing = document.getElementById(WIDGET_SCRIPT_ID)
    if (existing instanceof HTMLScriptElement) {
      if (existing.dataset.widgetKey === widgetKey) return
      existing.remove()
    }

    const script = document.createElement('script')
    script.id = WIDGET_SCRIPT_ID
    script.defer = true
    script.dataset.widgetKey = widgetKey
    script.setAttribute('data-widget-key', widgetKey)
    script.setAttribute('data-host', WIDGET_HOST)
    script.src = WIDGET_SCRIPT_SRC
    let timeoutID: ReturnType<typeof setTimeout> | undefined
    let idleID: number | undefined
    const appendScript = () => {
      if (!document.getElementById(WIDGET_SCRIPT_ID)) {
        document.body.appendChild(script)
      }
    }
    if ('requestIdleCallback' in window) {
      idleID = window.requestIdleCallback(appendScript, { timeout: 3000 })
    } else {
      timeoutID = globalThis.setTimeout(appendScript, 1500)
    }

    return () => {
      if (idleID !== undefined && 'cancelIdleCallback' in window) {
        window.cancelIdleCallback(idleID)
      }
      if (timeoutID !== undefined) globalThis.clearTimeout(timeoutID)
      script.remove()
    }
  }, [config.chat_widget_enabled, config.is_published, config.support_widget_key])

  return null
}

export function AppShell() {
  const [searchOpen, setSearchOpen] = useState(false)

  const openSearch = useCallback(() => setSearchOpen(true), [])
  const closeSearch = useCallback(() => setSearchOpen(false), [])

  // Global Cmd+K / Ctrl+K shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
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
      {searchOpen && (
        <Suspense fallback={null}>
          <SearchDialog open onClose={closeSearch} />
        </Suspense>
      )}
      <HelpCenterChatWidget />
    </div>
  )
}
