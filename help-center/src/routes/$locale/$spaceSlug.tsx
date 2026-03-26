import { useCallback, useState } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { Menu } from 'lucide-react'
import { SpaceProvider, useSpaceContext } from '@/contexts/SpaceContext'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$locale/$spaceSlug')({
  component: LocalizedSpaceRoute,
})

function LocalizedSpaceRoute() {
  const { spaceSlug } = Route.useParams()
  return (
    <SpaceProvider spaceSlug={spaceSlug}>
      <LocalizedSpaceLayout />
    </SpaceProvider>
  )
}

function LocalizedSpaceLayout() {
  const { locale } = Route.useParams()
  const { space, spaceSlug, navigation, isLoading } = useSpaceContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  if (isLoading) {
    return <LoadingState message="Loading space..." />
  }

  return (
    <div className="flex">
      <div className="fixed left-0 right-0 top-[var(--hc-header-height)] z-20 flex items-center gap-2 border-b border-border bg-background px-4 py-2 lg:hidden">
        <button
          onClick={() => setMobileNavOpen(true)}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
          aria-label="Open navigation"
        >
          <Menu size={18} />
        </button>
        {space && (
          <span className="truncate text-[13px] font-medium">{space.name}</span>
        )}
      </div>

      {mobileNavOpen && (
        <MobileNav
          locale={locale}
          navigation={navigation}
          spaceSlug={spaceSlug}
          onClose={closeMobileNav}
        />
      )}

      <Sidebar locale={locale} navigation={navigation} spaceSlug={spaceSlug} />

      <main className="min-w-0 flex-1 pt-[41px] lg:pt-0">
        <Outlet />
      </main>
    </div>
  )
}
