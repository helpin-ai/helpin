import { useRouter } from '@tanstack/react-router'
import { useUnreadStats } from '@helpin-ai/support-core'
import { useSupportViewStore } from '@mobile/stores/support-view-store'
import { TabBar, type TabKey } from './tab-bar'

interface PrimaryNavigationProps {
  activeTab: TabKey
  workspaceId: string
  workspaceSlug: string
}

/** Shared wiring for the four thumb-friendly mobile root destinations. */
export function PrimaryNavigation({ activeTab, workspaceId, workspaceSlug }: PrimaryNavigationProps) {
  const router = useRouter()
  const setSelection = useSupportViewStore((state) => state.setSelection)
  const { data: unread } = useUnreadStats(workspaceId)

  const handleNavigate = (tab: TabKey) => {
    if (tab === 'inbox' || tab === 'mine') {
      setSelection({ kind: 'builtin', navFilter: tab, mailboxId: 'all' })
      if (activeTab !== tab) {
        router.navigate({
          to: '/w/$slug/support',
          params: { slug: workspaceSlug },
        })
      }
      return
    }

    if (tab === 'search') {
      if (activeTab !== tab) {
        router.navigate({
          to: '/w/$slug/support/search',
          params: { slug: workspaceSlug },
        })
      }
      return
    }

    if (activeTab !== tab) {
      router.navigate({
        to: '/w/$slug/settings',
        params: { slug: workspaceSlug },
      })
    }
  }

  return (
    <TabBar
      activeTab={activeTab}
      onNavigate={handleNavigate}
      badges={{ inbox: unread?.inbox ?? 0, mine: unread?.mine ?? 0 }}
    />
  )
}
