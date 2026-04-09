import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { ThemeProvider } from 'next-themes'
import { ConfirmProvider } from '@/components/ui/confirm-dialog'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
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
          <Outlet />
          <Toaster closeButton />
        </ConfirmProvider>
      </TooltipProvider>
    </ThemeProvider>
  )
}
