import { useMemo, useState } from 'react'
import { InboxIcon } from '@/lib/icons'
import { toast } from 'sonner'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { usePendingSuggestions, useAcceptSuggestion, useDismissSuggestion } from '@/hooks/queries'
import { SuggestionCard } from '@/components/crm/SuggestionCard'
import { useTitle } from '@/hooks/useTitle'
import type { CRMSuggestionType } from '@/lib/crmTypes'

type FilterTab = 'all' | CRMSuggestionType

const tabs: { key: FilterTab; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'deal_create', label: 'Deal Create' },
  { key: 'deal_advance', label: 'Deal Advance' },
  { key: 'follow_up', label: 'Follow Up' },
  { key: 'risk_alert', label: 'Risk Alert' },
]

export function ReviewFeed() {
  useTitle('Review Feed')
  const { currentWorkspace } = useWorkspaceStore()
  const wsId = currentWorkspace?.id ?? ''
  const [activeTab, setActiveTab] = useState<FilterTab>('all')

  const { data, isLoading } = usePendingSuggestions(wsId)
  const acceptMutation = useAcceptSuggestion(wsId)
  const dismissMutation = useDismissSuggestion(wsId)

  const suggestions = useMemo(() => {
    const items = data?.data ?? []
    const filtered = activeTab === 'all' ? items : items.filter(s => s.suggestion_type === activeTab)
    return [...filtered].sort((a, b) => b.confidence - a.confidence)
  }, [data, activeTab])

  const handleAccept = (id: string) => {
    acceptMutation.mutate({ id }, {
      onSuccess: () => toast.success('Suggestion approved'),
      onError: (err) => toast.error(`Failed to approve: ${err.message}`),
    })
  }

  const handleDismiss = (id: string) => {
    dismissMutation.mutate(id, {
      onSuccess: () => toast.success('Suggestion dismissed'),
      onError: (err) => toast.error(`Failed to dismiss: ${err.message}`),
    })
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex flex-wrap items-center gap-2 border-b border-border/70 px-3 py-2">
        <div>
          <h1 className="text-sm font-semibold">Review Feed</h1>
          <p className="text-xs text-muted-foreground">AI-generated suggestions awaiting your review</p>
        </div>
      </header>

      {/* Filter tabs */}
      <div className="flex items-center gap-1 border-b border-border/70 px-3 py-1.5">
        {tabs.map(tab => (
          <button
            key={tab.key}
            type="button"
            onClick={() => setActiveTab(tab.key)}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              activeTab === tab.key
                ? 'bg-foreground/10 text-foreground'
                : 'text-muted-foreground hover:bg-muted/80 hover:text-foreground'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto p-3">
        {isLoading ? (
          <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
            Loading suggestions...
          </div>
        ) : suggestions.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <InboxIcon className="h-10 w-10 text-muted-foreground/40 mb-3" />
            <h3 className="text-sm font-medium text-muted-foreground">No pending suggestions</h3>
            <p className="mt-1 text-xs text-muted-foreground/70">
              {activeTab === 'all'
                ? 'When the AI detects deal opportunities or progression signals, they will appear here.'
                : `No ${activeTab.replace('_', ' ')} suggestions at the moment.`}
            </p>
          </div>
        ) : (
          <div className="space-y-2 max-w-2xl">
            {suggestions.map(suggestion => (
              <SuggestionCard
                key={suggestion.id}
                suggestion={suggestion}
                onAccept={handleAccept}
                onDismiss={handleDismiss}
                isAccepting={acceptMutation.isPending}
                isDismissing={dismissMutation.isPending}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
