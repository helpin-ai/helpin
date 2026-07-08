import { useParams } from '@tanstack/react-router'
import { TopBar } from '@mobile/ui/top-bar'
import { TabShell } from '@mobile/navigation/tab-bar'

export function InboxScreen() {
  const { slug } = useParams({ strict: false })

  // workspaceId is unresolved until Task 11 wires the real workspace lookup;
  // TabShell/useUnreadStats stay inert (enabled: !!workspaceId) until then.
  return (
    <TabShell workspaceSlug={slug ?? ''} workspaceId="">
      <TopBar title="Inbox" />
    </TabShell>
  )
}
