import { useState, useCallback } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { Menu } from 'lucide-react'
import { useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$spaceSlug')({
  component: SpaceLayout,
})

function SpaceLayout() {
  const { spaceSlug } = Route.useParams()
  const { subdomain, spaces } = useDocsContext()
  const { data: navigation, isLoading } = useSpaceNavigation(
    subdomain,
    spaceSlug,
  )
  const [mobileNavOpen, setMobileNavOpen] = useState(false)

  const currentSpace = spaces.find((s) => s.slug === spaceSlug)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  if (isLoading) {
    return <LoadingState message="Loading space..." />
  }

  return (
    <div className="flex">
      {/* Mobile nav bar — visible below header on small screens */}
      <div
        className="lg:hidden fixed top-[var(--hc-header-height)] left-0 right-0 z-20 flex items-center gap-2 px-4 py-2 border-b"
        style={{
          backgroundColor: 'var(--hc-bg)',
          borderColor: 'var(--hc-border)',
        }}
      >
        <button
          onClick={() => setMobileNavOpen(true)}
          className="p-1 -ml-1 rounded-md transition-colors hover:bg-[var(--hc-bg-secondary)]"
          aria-label="Open navigation"
        >
          <Menu size={18} style={{ color: 'var(--hc-text-secondary)' }} />
        </button>
        {currentSpace && (
          <span className="text-sm font-medium truncate">
            {currentSpace.name}
          </span>
        )}
      </div>

      {/* Mobile nav drawer */}
      {mobileNavOpen && (
        <MobileNav
          navigation={navigation ?? []}
          spaceSlug={spaceSlug}
          onClose={closeMobileNav}
        />
      )}

      {/* Desktop sidebar */}
      <Sidebar navigation={navigation ?? []} spaceSlug={spaceSlug} />

      {/* Main content — top padding on mobile for the nav bar */}
      <main className="flex-1 min-w-0 pt-[41px] lg:pt-0">
        <Outlet />
      </main>
    </div>
  )
}
