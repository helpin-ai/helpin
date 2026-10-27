import { lazy, Suspense, useState, useEffect, useCallback } from 'react'
import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import { useDocsContext } from '@/contexts/DocsContext'
import { OPEN_ASK_AI_EVENT } from '@/lib/askAI'
import type { AskAIRequest } from '@/components/assistant/AskAIPanel'

const SearchDialog = lazy(async () => {
  const module = await import('@/components/search/SearchDialog')
  return { default: module.SearchDialog }
})

const AskAIPanel = lazy(async () => {
  const module = await import('@/components/assistant/AskAIPanel')
  return { default: module.AskAIPanel }
})

const WIDGET_SCRIPT_ID = 'helpin-widget'

function HelpCenterChatWidget() {
  const { config } = useDocsContext()

  useEffect(() => {
    const widgetKey = config.support_widget_key?.trim()
    const host = config.public_widget_url || import.meta.env.VITE_WIDGET_HOST
    const scriptURL = config.public_sdk_url || import.meta.env.VITE_WIDGET_SCRIPT_URL
    if (!host || !scriptURL) return
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
    script.setAttribute('data-host', host)
    script.setAttribute('data-support-only', 'true')
    script.src = scriptURL
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
  }, [config.chat_widget_enabled, config.is_published, config.support_widget_key, config.public_widget_url, config.public_sdk_url])

  return null
}

export function AppShell() {
  const { config } = useDocsContext()
  const [searchOpen, setSearchOpen] = useState(false)
  const assistantEnabled = config.ai_assistant_enabled === true
  // null: closed. A request object: open, possibly with a question to ask.
  const [askAI, setAskAI] = useState<AskAIRequest | null>(null)

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

  // Ask AI opens with Cmd+I / Ctrl+I, or from a page or the search dialog.
  useEffect(() => {
    if (!assistantEnabled) return
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'i') {
        e.preventDefault()
        setSearchOpen(false)
        setAskAI((current) => current ?? {})
      }
    }
    const onOpen = (e: Event) => {
      setSearchOpen(false)
      setAskAI({ ...((e as CustomEvent<AskAIRequest>).detail ?? {}) })
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener(OPEN_ASK_AI_EVENT, onOpen)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener(OPEN_ASK_AI_EVENT, onOpen)
    }
  }, [assistantEnabled])

  // Allow child components (e.g. homepage) to open search via custom event
  useEffect(() => {
    const handler = () => setSearchOpen(true)
    window.addEventListener('open-help-search', handler)
    return () => window.removeEventListener('open-help-search', handler)
  }, [])

  return (
    <div className="min-h-screen flex flex-col">
      <TopBar onSearchClick={openSearch} onAskAIClick={assistantEnabled ? () => setAskAI((current) => current ?? {}) : undefined} />
      <div className="flex-1">
        <Outlet />
      </div>
      {searchOpen && (
        <Suspense fallback={null}>
          <SearchDialog open onClose={closeSearch} />
        </Suspense>
      )}
      {askAI && (
        <Suspense fallback={null}>
          <AskAIPanel request={askAI} onClose={() => setAskAI(null)} />
        </Suspense>
      )}
      <HelpCenterChatWidget />
    </div>
  )
}
