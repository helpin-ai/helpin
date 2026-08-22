import { useMemo, useState } from 'react'
import { Bot, CheckCircle2, ChevronRight, Clock3, MessageSquareText, ShieldCheck, XCircle } from 'lucide-react'
import {
  useAgentRunMessages,
  useApproveAgentRun,
  useConversationAgentRuns,
  type SupportAgentRun,
} from '@helpin-ai/support-core'
import { toast } from 'sonner'

import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@mobile/lib/upgrade-required'
import { cn } from '@mobile/lib/cn'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'
import { UpgradeRequiredSheet } from '@mobile/ui/upgrade-required-sheet'
import {
  agentRunStatusLabel,
  agentRunSummary,
  formatAgentRunTimestamp,
  orderAgentRuns,
} from './agent-runs'

export function AssignedAgentRuns({ workspaceId, conversationId, enabled, canApprove }: {
  workspaceId: string
  conversationId: string
  enabled: boolean
  canApprove: boolean
}) {
  const runsQuery = useConversationAgentRuns(workspaceId, conversationId, enabled)
  const approve = useApproveAgentRun(workspaceId, conversationId)
  const [selectedRun, setSelectedRun] = useState<SupportAgentRun | null>(null)
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null)
  const runs = useMemo(() => orderAgentRuns(runsQuery.data ?? []), [runsQuery.data])
  const pendingRun = runs.find((run) => run.approval_state === 'pending') ?? null

  if (!enabled || runs.length === 0) return null

  const approveDraft = async (runId: string) => {
    try {
      await approve.mutateAsync(runId)
      toast.success('Draft approved and sent')
    } catch (error) {
      const reason = getUpgradeRequiredReason(error)
      if (reason) setUpgradeReason(reason)
      else toast.error('Could not approve agent draft')
    }
  }

  return (
    <>
      <section aria-label="Assigned agent activity" className="border-b border-border/60 bg-muted/20 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
              <Bot className="h-4 w-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-footnote font-semibold text-foreground">Assigned agent activity</h2>
              <p className="text-caption text-muted-foreground">{runs.length} {runs.length === 1 ? 'run' : 'runs'} on this conversation</p>
            </div>
          </div>
          {pendingRun && canApprove && (
            <Pressable
              disabled={approve.isPending}
              onPress={() => void approveDraft(pendingRun.id)}
              className="flex min-h-9 w-auto min-w-0 shrink-0 items-center gap-1.5 rounded-full bg-primary px-3 text-footnote font-semibold text-primary-foreground disabled:opacity-50"
            >
              {approve.isPending ? <Spinner size={14} /> : <ShieldCheck className="h-4 w-4" />}
              Approve draft
            </Pressable>
          )}
        </div>

        <div className="mt-3 space-y-2">
          {runs.map((run) => (
            <Pressable
              key={run.id}
              aria-label={`Open ${agentRunStatusLabel(run.status)} agent run details`}
              onPress={() => setSelectedRun(run)}
              className="flex min-h-12 w-full items-center gap-3 rounded-2xl border border-border/70 bg-background px-3 py-2 text-left"
            >
              <RunStatusIcon run={run} />
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2 text-footnote font-medium text-foreground">
                  {agentRunStatusLabel(run.status)}
                  {run.invocation_mode === 'interactive' && (
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">Interactive</span>
                  )}
                </span>
                <span className="mt-0.5 block truncate text-caption text-muted-foreground">
                  {run.approval_state === 'pending' ? 'Draft waiting for approval' : formatAgentRunTimestamp(run.created_at)}
                </span>
              </span>
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            </Pressable>
          ))}
        </div>
      </section>

      <AgentRunDetailsSheet
        workspaceId={workspaceId}
        run={selectedRun}
        open={!!selectedRun}
        onOpenChange={(open) => { if (!open) setSelectedRun(null) }}
      />
      <UpgradeRequiredSheet reason={upgradeReason} onOpenChange={(open) => { if (!open) setUpgradeReason(null) }} />
    </>
  )
}

function AgentRunDetailsSheet({ workspaceId, run, open, onOpenChange }: {
  workspaceId: string
  run: SupportAgentRun | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const messagesQuery = useAgentRunMessages(workspaceId, run?.id ?? null, open)
  const messages = (messagesQuery.data ?? []).filter((message) => message.content.trim())
  const summary = run ? agentRunSummary(run) : null
  if (!run) return null

  const milestones = [
    { label: 'Created', value: run.created_at },
    { label: 'Started', value: run.started_at },
    { label: 'Completed', value: run.completed_at },
  ].filter((item) => item.value)

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Agent run details">
      <div className="min-h-0 overflow-y-auto px-5 pb-4">
        <div className="flex items-start gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"><Bot className="h-5 w-5" /></span>
          <div className="min-w-0 flex-1">
            <h2 className="text-headline text-foreground">Agent run</h2>
            <div className="mt-1 flex flex-wrap gap-1.5">
              <span className="rounded-full bg-muted px-2 py-0.5 text-caption font-medium">{agentRunStatusLabel(run.status)}</span>
              <span className="rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground">{agentRunStatusLabel(run.approval_state)}</span>
              {run.execution_stage && <span className="rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground">{agentRunStatusLabel(run.execution_stage)}</span>}
            </div>
          </div>
        </div>

        {summary && (
          <div className={cn('mt-4 rounded-2xl border p-3', run.status === 'failed' ? 'border-red-300/70 bg-red-50/60 dark:border-red-900/60 dark:bg-red-950/20' : 'border-border/70 bg-muted/30')}>
            <p className="whitespace-pre-wrap text-footnote text-foreground">{summary}</p>
          </div>
        )}

        <div className="mt-5">
          <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">Status history</h3>
          <div className="mt-2 space-y-3">
            {milestones.map((item) => (
              <div key={item.label} className="flex items-center gap-3 text-footnote">
                <Clock3 className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="font-medium text-foreground">{item.label}</span>
                <span className="ml-auto text-right text-muted-foreground">{formatAgentRunTimestamp(item.value)}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-5">
          <h3 className="flex items-center gap-2 text-caption font-semibold uppercase tracking-wide text-muted-foreground"><MessageSquareText className="h-4 w-4" />Run messages</h3>
          {messagesQuery.isPending ? (
            <div className="flex justify-center py-8"><Spinner /></div>
          ) : messagesQuery.isError ? (
            <p className="py-4 text-footnote text-muted-foreground">Run messages could not be loaded.</p>
          ) : messages.length === 0 ? (
            <p className="py-4 text-footnote text-muted-foreground">No readable run messages yet.</p>
          ) : (
            <div className="mt-2 space-y-2">
              {messages.map((message) => (
                <div key={message.id} className="rounded-2xl border border-border/70 bg-background p-3">
                  <p className="text-caption font-semibold capitalize text-muted-foreground">{message.role}</p>
                  <p className="mt-1 whitespace-pre-wrap break-words text-footnote text-foreground">{message.content}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </Sheet>
  )
}

function RunStatusIcon({ run }: { run: SupportAgentRun }) {
  if (run.status === 'completed') return <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-500" />
  if (run.status === 'failed' || run.status === 'cancelled') return <XCircle className="h-5 w-5 shrink-0 text-red-500" />
  return <Clock3 className="h-5 w-5 shrink-0 text-amber-500" />
}
