import { useEffect, useMemo, useState } from 'react'
import { Outlet, useRouterState } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { isTauriDesktop } from '@desktop/lib/desktopHost'
import {
  closeDesktopWindow,
  isDesktopWindowMaximized,
  minimizeDesktopWindow,
  onDesktopWindowResized,
  toggleDesktopWindowMaximize,
} from '@desktop/lib/desktopWindow'

function useIsMacPlatform() {
  return useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /mac/i.test(navigator.userAgent)
  }, [])
}

function getWindowTitle(pathname: string) {
  if (pathname.startsWith('/login')) return 'Sign In'
  if (pathname.startsWith('/workspaces')) return 'Workspaces'
  if (pathname.includes('/support')) return 'Support'
  if (pathname.includes('/crm/') || pathname.includes('/docs/') || pathname.includes('/pm/')) {
    return 'Open In Browser'
  }
  return 'Helpin Support'
}

function WindowsControlIcon({ kind, maximized }: { kind: 'minimize' | 'maximize' | 'close'; maximized: boolean }) {
  if (kind === 'minimize') {
    return <span className="block h-px w-3 bg-current" />
  }

  if (kind === 'close') {
    return (
      <span className="relative block h-3 w-3">
        <span className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 rotate-45 bg-current" />
        <span className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 -rotate-45 bg-current" />
      </span>
    )
  }

  if (maximized) {
    return (
      <span className="relative block h-3 w-3">
        <span className="absolute right-0 top-0 h-2.5 w-2.5 border border-current bg-transparent" />
        <span className="absolute left-0 top-0.5 h-2.5 w-2.5 border border-current bg-transparent" />
      </span>
    )
  }

  return <span className="block h-3 w-3 border border-current bg-transparent" />
}

function MacWindowControls() {
  return (
    <div className="flex items-center gap-2 px-3">
      <button
        type="button"
        aria-label="Close window"
        className="h-3 w-3 rounded-full bg-[#ff5f57] ring-1 ring-black/10 transition hover:brightness-95"
        onClick={() => { void closeDesktopWindow() }}
      />
      <button
        type="button"
        aria-label="Minimize window"
        className="h-3 w-3 rounded-full bg-[#febc2e] ring-1 ring-black/10 transition hover:brightness-95"
        onClick={() => { void minimizeDesktopWindow() }}
      />
      <button
        type="button"
        aria-label="Toggle maximize"
        className="h-3 w-3 rounded-full bg-[#28c840] ring-1 ring-black/10 transition hover:brightness-95"
        onClick={() => { void toggleDesktopWindowMaximize() }}
      />
    </div>
  )
}

function WindowsWindowControls({ maximized }: { maximized: boolean }) {
  return (
    <div className="flex items-stretch">
      <button
        type="button"
        aria-label="Minimize window"
        className="flex h-10 w-12 items-center justify-center text-foreground/70 transition hover:bg-black/5 hover:text-foreground dark:hover:bg-white/10"
        onClick={() => { void minimizeDesktopWindow() }}
      >
        <WindowsControlIcon kind="minimize" maximized={maximized} />
      </button>
      <button
        type="button"
        aria-label={maximized ? 'Restore window' : 'Maximize window'}
        className="flex h-10 w-12 items-center justify-center text-foreground/70 transition hover:bg-black/5 hover:text-foreground dark:hover:bg-white/10"
        onClick={() => { void toggleDesktopWindowMaximize() }}
      >
        <WindowsControlIcon kind="maximize" maximized={maximized} />
      </button>
      <button
        type="button"
        aria-label="Close window"
        className="flex h-10 w-12 items-center justify-center text-foreground/70 transition hover:bg-red-500 hover:text-white"
        onClick={() => { void closeDesktopWindow() }}
      >
        <WindowsControlIcon kind="close" maximized={maximized} />
      </button>
    </div>
  )
}

export function DesktopWindowFrame() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const isMac = useIsMacPlatform()
  const [maximized, setMaximized] = useState(false)
  const title = getWindowTitle(pathname)

  useEffect(() => {
    if (!isTauriDesktop()) return

    let mounted = true
    let unsubscribe = () => {}

    async function setup() {
      const sync = async () => {
        const next = await isDesktopWindowMaximized()
        if (mounted) {
          setMaximized(next)
        }
      }

      await sync()
      unsubscribe = await onDesktopWindowResized(() => {
        void sync()
      })
    }

    void setup()

    return () => {
      mounted = false
      unsubscribe()
    }
  }, [])

  return (
    <div className="flex min-h-screen flex-col bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.92)_45%,rgba(187,210,229,0.55)_100%)] text-foreground">
      <div className="flex h-10 shrink-0 items-center border-b border-border/70 bg-background/88 backdrop-blur supports-[backdrop-filter]:bg-background/72">
        {isMac ? <MacWindowControls /> : <div className="w-3" />}
        <div
          data-tauri-drag-region
          className="flex min-w-0 flex-1 items-center justify-center gap-2 px-3"
        >
          <div className="h-2 w-2 rounded-full bg-primary/70" />
          <span className="truncate text-[12px] font-medium tracking-[0.16em] text-muted-foreground uppercase">
            {title}
          </span>
        </div>
        {isMac ? (
          <div className="w-[72px]" />
        ) : (
          <WindowsWindowControls maximized={maximized} />
        )}
      </div>

      <div className={cn('min-h-0 flex-1 overflow-hidden', !isTauriDesktop() && 'border-t border-transparent')}>
        <Outlet />
      </div>
    </div>
  )
}
