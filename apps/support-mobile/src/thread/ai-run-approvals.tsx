import { useState } from 'react'
import { Bot, Check, ShieldCheck } from 'lucide-react'
import {
  useConversationAIRunInteractions,
  useResolveConversationAIRunInteraction,
  type SupportRunInteraction,
} from '@helpin-ai/support-core'
import { toast } from 'sonner'
import { cn } from '@mobile/lib/cn'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import {
  buildQuestionResponse,
  interactionId,
  parseInteractionQuestions,
  questionsAnswered,
} from './ai-interactions'

export function AIRunApprovals({ workspaceId, conversationId, enabled }: {
  workspaceId: string
  conversationId: string
  enabled: boolean
}) {
  const interactionsQuery = useConversationAIRunInteractions(workspaceId, conversationId, enabled)
  const resolve = useResolveConversationAIRunInteraction(workspaceId, conversationId)
  if (!enabled) return null
  return (
    <RunInteractionCards
      interactions={interactionsQuery.data?.interactions ?? []}
      onResolve={(variables, options) => resolve.mutate(variables, options)}
      resolving={resolve.isPending}
    />
  )
}

interface ResolveInteractionVariables {
  interactionId: string
  payload: { response_payload: Record<string, unknown>; followup_message?: string }
}

export function RunInteractionCards({ interactions, onResolve, resolving, inline = false }: {
  interactions: SupportRunInteraction[]
  onResolve: (
    variables: ResolveInteractionVariables,
    options: { onSuccess: () => void; onError: () => void },
  ) => void
  resolving: boolean
  inline?: boolean
}) {
  const pending = interactions.filter((item) => item.status === 'pending' && interactionId(item))
  if (pending.length === 0) return null
  return (
    <div className={cn('space-y-2', !inline && 'border-b border-amber-200/70 bg-amber-50/50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/20')}>
      <div className="flex items-center gap-2 text-footnote font-semibold"><ShieldCheck className="h-4 w-4 text-amber-600" />Agent needs your input</div>
      {pending.map((interaction) => (
        <InteractionCard
          key={interactionId(interaction)}
          interaction={interaction}
          onResolve={onResolve}
          resolving={resolving}
        />
      ))}
    </div>
  )
}

function InteractionCard({ interaction, onResolve, resolving }: {
  interaction: SupportRunInteraction
  onResolve: (
    variables: ResolveInteractionVariables,
    options: { onSuccess: () => void; onError: () => void },
  ) => void
  resolving: boolean
}) {
  const [note, setNote] = useState('')
  const [answers, setAnswers] = useState<Record<string, { value?: string; freetext?: string }>>({})
  const [selectedFindingIds, setSelectedFindingIds] = useState<string[]>([])
  const questions = parseInteractionQuestions(interaction.request_payload)
  const id = interactionId(interaction)
  const submit = (responsePayload: Record<string, unknown>) => onResolve(
    { interactionId: id, payload: { response_payload: responsePayload, ...(note.trim() ? { followup_message: note.trim() } : {}) } },
    { onSuccess: () => toast.success('Response sent'), onError: () => toast.error('Could not send response') },
  )
  const runtime = interaction.request_payload
  const requestedPermissions = runtime.permissions && typeof runtime.permissions === 'object' ? runtime.permissions as Record<string, unknown> : {}
  const findings = Array.isArray(runtime.findings) ? runtime.findings.flatMap((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return []
    const finding = item as Record<string, unknown>
    if (typeof finding.id !== 'string' || typeof finding.title !== 'string') return []
    return [{ id: finding.id, title: finding.title, body: typeof finding.body === 'string' ? finding.body : '', priority: typeof finding.priority === 'string' ? finding.priority : '' }]
  }) : []
  const isRuntimeApproval = interaction.interaction_kind === 'command_execution_approval' || interaction.interaction_kind === 'file_change_approval'
  const decisions = Array.isArray(runtime.decisions) ? runtime.decisions.filter((item): item is string => typeof item === 'string') : ['accept', 'acceptForSession', 'decline', 'cancel']
  const title = interaction.title || interaction.interaction_kind.replaceAll('_', ' ')

  return (
    <section aria-label={title} className="rounded-2xl border border-amber-300/60 bg-background p-3 shadow-sm dark:border-amber-900/70">
      <div className="flex items-start gap-2"><Bot className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><div className="min-w-0"><h3 className="text-footnote font-semibold capitalize">{title}</h3>{interaction.summary && <p className="mt-1 whitespace-pre-wrap text-footnote text-muted-foreground">{interaction.summary}</p>}</div></div>

      {interaction.interaction_kind === 'request_user_input' && questions.length > 0 && (
        <div className="mt-3 space-y-4">
          {questions.map((question) => {
            const answer = answers[question.id]
            const needsText = question.options.length === 0 || answer?.value === '__other__' || question.options.find((option) => option.value === answer?.value)?.freetext
            return <div key={question.id} className="space-y-2"><p className="text-footnote font-medium">{question.prompt}</p>
              {question.options.map((option) => <Pressable key={option.value} aria-pressed={answer?.value === option.value} onPress={() => setAnswers((current) => ({ ...current, [question.id]: { value: option.value } }))} className={cn('flex min-h-10 items-center gap-2 rounded-xl border px-3 text-left text-footnote', answer?.value === option.value ? 'border-primary/40 bg-primary/5' : 'border-border')}><span className="min-w-0 flex-1"><span className="block font-medium">{option.label}</span>{option.description && <span className="block text-caption text-muted-foreground">{option.description}</span>}</span>{answer?.value === option.value && <Check className="h-4 w-4 text-primary" />}</Pressable>)}
              {question.allowOther && <Pressable aria-pressed={answer?.value === '__other__'} onPress={() => setAnswers((current) => ({ ...current, [question.id]: { value: '__other__' } }))} className={cn('flex min-h-10 items-center rounded-xl border px-3 text-footnote', answer?.value === '__other__' ? 'border-primary/40 bg-primary/5' : 'border-border')}>Other</Pressable>}
              {needsText && <input type={question.secret ? 'password' : 'text'} value={answer?.freetext ?? ''} onChange={(event) => setAnswers((current) => ({ ...current, [question.id]: { ...current[question.id], freetext: event.target.value } }))} placeholder="Type your answer" className="h-11 w-full rounded-xl border border-input bg-background px-3 text-body outline-none" />}
            </div>
          })}
          <ActionButton disabled={!questionsAnswered(questions, answers)} busy={resolving} onPress={() => submit(buildQuestionResponse(questions, answers))}>Submit answers</ActionButton>
        </div>
      )}

      {interaction.interaction_kind !== 'request_user_input' && interaction.interaction_kind !== 'auth_required' && (
        <div className="mt-3 space-y-3">
          {(isRuntimeApproval || interaction.interaction_kind === 'permissions_approval') && <pre className="max-h-40 overflow-auto whitespace-pre-wrap rounded-xl bg-slate-950 p-3 text-[11px] text-slate-100">{JSON.stringify(runtime, null, 2)}</pre>}
          {interaction.interaction_kind === 'review_checkpoint' && findings.length > 0 && <div className="space-y-1"><p className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">Select findings to continue</p>{findings.map((finding) => { const selected = selectedFindingIds.includes(finding.id); return <Pressable key={finding.id} aria-pressed={selected} onPress={() => setSelectedFindingIds((current) => selected ? current.filter((id) => id !== finding.id) : [...current, finding.id])} className={cn('flex min-h-11 items-start gap-2 rounded-xl border p-3 text-left', selected ? 'border-primary/40 bg-primary/5' : 'border-border')}><span className="min-w-0 flex-1"><span className="block text-footnote font-medium">{finding.title}</span>{finding.body && <span className="mt-0.5 block text-caption text-muted-foreground">{finding.body}</span>}</span>{finding.priority && <span className="rounded-full border border-border px-1.5 py-0.5 text-[10px] uppercase">{finding.priority}</span>}{selected && <Check className="h-4 w-4 shrink-0 text-primary" />}</Pressable> })}</div>}
          {!isRuntimeApproval && interaction.interaction_kind !== 'permissions_approval' && <textarea value={note} onChange={(event) => setNote(event.target.value)} placeholder="Optional note" className="min-h-20 w-full rounded-xl border border-input bg-background p-3 text-body outline-none" />}
          <div className="flex flex-wrap gap-2">
            {(interaction.interaction_kind === 'approval_request' || interaction.interaction_kind === 'review_checkpoint') && <><ActionButton disabled={interaction.interaction_kind === 'review_checkpoint' && findings.length > 0 && selectedFindingIds.length === 0} busy={resolving} onPress={() => submit({ decision: 'approve', ...(interaction.interaction_kind === 'review_checkpoint' && findings.length > 0 ? { selection_mode: 'selected', selected_finding_ids: selectedFindingIds } : {}) })}>Approve</ActionButton><ActionButton secondary busy={resolving} onPress={() => submit({ decision: 'request_changes', ...(interaction.interaction_kind === 'review_checkpoint' && findings.length > 0 ? { selection_mode: selectedFindingIds.length > 0 ? 'selected' : 'all', ...(selectedFindingIds.length > 0 ? { selected_finding_ids: selectedFindingIds } : {}) } : {}) })}>Request changes</ActionButton>{interaction.interaction_kind === 'review_checkpoint' && findings.length > 0 && <ActionButton secondary busy={resolving} onPress={() => submit({ decision: 'skip', selection_mode: 'none', selected_finding_ids: [] })}>Skip</ActionButton>}</>}
            {interaction.interaction_kind === 'permissions_approval' && <><ActionButton busy={resolving} onPress={() => submit({ permissions: requestedPermissions, scope: 'turn' })}>Allow for turn</ActionButton><ActionButton secondary busy={resolving} onPress={() => submit({ permissions: requestedPermissions, scope: 'session' })}>Allow for session</ActionButton><ActionButton secondary busy={resolving} onPress={() => submit({ permissions: {}, scope: 'turn' })}>Deny</ActionButton></>}
            {isRuntimeApproval && decisions.map((decision) => <ActionButton key={decision} secondary={!decision.startsWith('accept')} busy={resolving} onPress={() => submit({ decision })}>{decision === 'acceptForSession' ? 'Allow for session' : decision === 'accept' ? 'Allow' : decision === 'decline' ? 'Deny' : 'Cancel'}</ActionButton>)}
          </div>
        </div>
      )}

      {interaction.interaction_kind === 'auth_required' && <pre className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap rounded-xl bg-slate-950 p-3 text-[11px] text-slate-100">{JSON.stringify(runtime, null, 2)}</pre>}
    </section>
  )
}

function ActionButton({ children, onPress, secondary, disabled, busy }: { children: string; onPress: () => void; secondary?: boolean; disabled?: boolean; busy?: boolean }) {
  return <Pressable disabled={disabled || busy} onPress={onPress} className={cn('flex min-h-9 w-auto min-w-0 items-center rounded-full px-4 text-footnote font-semibold disabled:opacity-40', secondary ? 'border border-border text-foreground' : 'bg-primary text-primary-foreground')}>{busy ? <Spinner size={14} /> : children}</Pressable>
}
