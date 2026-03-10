import { useState, useCallback } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { Menu } from 'lucide-react'
import { SpaceProvider, useSpaceContext } from '@/contexts/SpaceContext'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$spaceSlug')({
  component: SpaceRoute,
})

function SpaceRoute() {
  const { spaceSlug } = Route.useParams()
  return (
    <SpaceProvider spaceSlug={spaceSlug}>
      <SpaceLayout />
    </SpaceProvider>
  )
}

function SpaceLayout() {
  const { space, spaceSlug, navigation, isLoading } = useSpaceContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  if (isLoading) {
    return <LoadingState message="Loading space..." />
  }

  return (
    <div className="flex">
      {/* Mobile nav bar */}
      <div className="lg:hidden fixed top-[var(--hc-header-height)] left-0 right-0 z-20 flex items-center gap-2 px-4 py-2 border-b border-border bg-background">
        <button
          onClick={() => setMobileNavOpen(true)}
          className="inline-flex items-center justify-center h-7 w-7 -ml-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
          aria-label="Open navigation"
        >
          <Menu size={18} />
        </button>
        {space && (
          <span className="text-[13px] font-medium truncate">{space.name}</span>
        )}
      </div>

      {/* Mobile nav drawer */}
      {mobileNavOpen && (
        <MobileNav
          navigation={navigation}
          spaceSlug={spaceSlug}
          onClose={closeMobileNav}
        />
      )}

      {/* Desktop sidebar */}
      <Sidebar navigation={navigation} spaceSlug={spaceSlug} />

      {/* Main content */}
      <main className="flex-1 min-w-0 pt-[41px] lg:pt-0">
        <Outlet />
      </main>
    </div>
  )
}
