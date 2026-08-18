import { createRootRouteWithContext, Outlet, useLocation } from '@tanstack/react-router'
import { useLayoutEffect } from 'react'
import { ThemeProvider } from 'next-themes'
import { ConfirmProvider } from '@/components/ui/confirm-dialog'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { HelpinWidgetVisibility } from '@/components/HelpinWidgetVisibility'
import { getPageTitle } from '@/lib/pageTitle'
import type { User } from '@/lib/types'

export interface RouterContext {
  auth: {
    user: User | null
    loading: boolean
    serverUnreachable: boolean
  }
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
  notFoundComponent: () => {
    return <div>Page not found</div>
  },
})

function RootComponent() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <TooltipProvider>
        <ConfirmProvider>
          <RouteTitle />
          <HelpinWidgetVisibility />
          <Outlet />
          <Toaster closeButton />
        </ConfirmProvider>
      </TooltipProvider>
    </ThemeProvider>
  )
}

function RouteTitle() {
  const { pathname } = useLocation()

  useLayoutEffect(() => {
    document.title = `${getPageTitle(pathname)} · Helpin`
  }, [pathname])

  return null
}
