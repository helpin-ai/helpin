import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import Markdown from 'react-markdown'
import { Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useChatSettings, useSupportAgents } from '@/hooks/queries/useSupport'
import { useWorkspaces } from '@/hooks/queries/useWorkspaces'
import { useAuthStore } from '@/stores/authStore'
import { unwrap } from '@/lib/queryUtils'
import { agentService } from '@/lib/services/agentService'
import { WorkspaceSelector } from '@/components/workspace-selector'
import type {
  MessageSenderType,
  SupportAIPreviewHistoryTurn,
  SupportAIPreviewResponse,
} from '@/lib/pmTypes'

type EditableHistoryTurn = SupportAIPreviewHistoryTurn & { id: string }

let turnIdCounter = 0

function createTurn(senderType: MessageSenderType = 'customer', content = ''): EditableHistoryTurn {
  turnIdCounter += 1
  return { id: `turn-${turnIdCounter}`, sender_type: senderType, message_type: 'reply', content }
}

const SENDER_OPTIONS: Array<{ value: MessageSenderType; label: string }> = [
  { value: 'customer', label: 'Customer' },
  { value: 'ai', label: 'AI' },
  { value: 'agent', label: 'Agent' },
  { value: 'user', label: 'Assistant' },
]

function normalizeResponse(data: SupportAIPreviewResponse): SupportAIPreviewResponse {
  const qp = data.query_plan ?? { decision: '', standalone_query: '', search_queries: [], clarifying_question: '', reason: '', tokens_used: 0, fallback_used: false }
  const ret = data.retrieval ?? { query_count: 0, result_count: 0, results: [] }
  return {
    ...data,
    query_plan: { ...qp, search_queries: qp.search_queries ?? [] },
    retrieval: { ...ret, results: ret.results ?? [] },
    answer: data.answer ? { ...data.answer, source_doc_ids: data.answer.source_doc_ids ?? [] } : undefined,
  }
}

export function ChatPlaygroundPage() {
  const user = useAuthStore((s) => s.user)
  const { data: workspaces = [], isLoading: workspacesLoading } = useWorkspaces()
  const [workspaceId, setWorkspaceId] = useState('')

  useEffect(() => {
    if (workspaces.length > 0 && !workspaceId) {
      const defaultWs = workspaces.find((w) => w.id === user?.default_workspace_id)
      setWorkspaceId(defaultWs?.id ?? workspaces[0].id)
    }
  }, [workspaces, user?.default_workspace_id, workspaceId])

  const { data: supportAgents = [], isLoading: agentsLoading } = useSupportAgents(workspaceId)
  const { data: chatSettings } = useChatSettings(workspaceId)

  const configuredAgentId = chatSettings?.settings.ai_agent_id ?? ''
  const [agentId, setAgentId] = useState('')
  const [conversationId, setConversationId] = useState('')
  const [latestMessage, setLatestMessage] = useState('')
  const [includeAnswer, setIncludeAnswer] = useState(true)
  const [maxResults, setMaxResults] = useState(8)
  const [history, setHistory] = useState<EditableHistoryTurn[]>([])
  const [response, setResponse] = useState<SupportAIPreviewResponse | null>(null)

  const configuredExists = configuredAgentId && supportAgents.some((a) => a.id === configuredAgentId)
  const selectedAgentId = agentId || (configuredExists ? configuredAgentId : supportAgents[0]?.id || '')

  const previewMutation = useMutation({
    mutationFn: async () => {
      if (!selectedAgentId) throw new Error('Select a support agent first.')
      if (!latestMessage.trim()) throw new Error('Latest message is required.')

      const turns = history
        .map((t) => ({ sender_type: t.sender_type, message_type: t.message_type || 'reply', content: t.content.trim() }))
        .filter((t) => t.content.length > 0)

      return unwrap(
        await agentService.previewSupportReply(workspaceId, selectedAgentId, {
          message: latestMessage.trim(),
          conversation_id: conversationId.trim() || undefined,
          history: turns.length > 0 ? turns : undefined,
          include_answer: includeAnswer,
          max_results: Math.max(1, Math.min(maxResults || 8, 12)),
        }),
      )
    },
    onSuccess: (data) => setResponse(normalizeResponse(data)),
    onError: (error: Error) => toast.error('Preview failed', { description: error.message }),
  })

  const updateTurn = (id: string, patch: Partial<SupportAIPreviewHistoryTurn>) =>
    setHistory((h) => h.map((t) => (t.id === id ? { ...t, ...patch } : t)))

  const handleWorkspaceChange = (id: string) => {
    setWorkspaceId(id)
    setAgentId('')
    setResponse(null)
  }

  const handleClear = () => {
    setConversationId('')
    setLatestMessage('')
    setHistory([])
    setResponse(null)
  }

  const searchQueries = response?.query_plan.search_queries ?? []
  const retrievalResults = response?.retrieval.results ?? []

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      {/* Form */}
      <div className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-1.5">
            <Label>Workspace</Label>
            <WorkspaceSelector
              workspaces={workspaces}
              value={workspaceId}
              onChange={handleWorkspaceChange}
              loading={workspacesLoading}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Agent</Label>
            <Select value={selectedAgentId} onValueChange={setAgentId} disabled={!workspaceId}>
              <SelectTrigger>
                <SelectValue placeholder={agentsLoading ? 'Loading...' : 'Select agent'} />
              </SelectTrigger>
              <SelectContent>
                {supportAgents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="space-y-1.5">
          <Label>Message</Label>
          <Textarea
            value={latestMessage}
            onChange={(e) => setLatestMessage(e.target.value)}
            placeholder="Customer message to test"
            rows={2}
          />
        </div>

        <div className="grid gap-4 md:grid-cols-[1fr_100px]">
          <div className="space-y-1.5">
            <Label>Conversation ID</Label>
            <Input
              value={conversationId}
              onChange={(e) => setConversationId(e.target.value)}
              placeholder="Optional: replay existing conversation"
            />
          </div>
          <div className="space-y-1.5">
            <Label>Max Results</Label>
            <Input
              type="number"
              min={1}
              max={12}
              value={maxResults}
              onChange={(e) => setMaxResults(Number(e.target.value) || 8)}
            />
          </div>
        </div>

        {/* Actions row */}
        <div className="flex items-center gap-4">
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={includeAnswer} onCheckedChange={setIncludeAnswer} />
            Generate answer
          </label>
          <Button type="button" variant="outline" size="sm" onClick={() => setHistory((h) => [...h, createTurn('customer')])}>
            <Plus className="mr-1 h-3.5 w-3.5" />
            Add Turn
          </Button>
          <div className="flex-1" />
          <Button type="button" variant="ghost" size="sm" onClick={handleClear}>
            Clear
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => previewMutation.mutate()}
            disabled={previewMutation.isPending || !workspaceId || !selectedAgentId || !latestMessage.trim()}
          >
            {previewMutation.isPending ? 'Running...' : 'Run Preview'}
          </Button>
        </div>

        {/* History turns */}
        {history.length > 0 && (
          <div className="space-y-2">
            {history.map((turn, i) => (
              <div key={turn.id} className="flex items-start gap-2 rounded-md border p-2">
                <Select
                  value={turn.sender_type}
                  onValueChange={(v) => updateTurn(turn.id, { sender_type: v as MessageSenderType })}
                >
                  <SelectTrigger className="w-[120px] shrink-0">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SENDER_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Textarea
                  value={turn.content}
                  onChange={(e) => updateTurn(turn.id, { content: e.target.value })}
                  placeholder={`Turn ${i + 1}`}
                  rows={1}
                  className="min-h-9"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="shrink-0"
                  onClick={() => setHistory((h) => h.filter((t) => t.id !== turn.id))}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Results */}
      {response && (
        <div className="space-y-6 border-t pt-6">
          {/* Outcome badges */}
          <div className="flex flex-wrap gap-2">
            <Badge
              variant={
                response.final_decision === 'answer'
                  ? 'default'
                  : response.final_decision === 'handoff'
                    ? 'destructive'
                    : 'secondary'
              }
            >
              {response.final_decision}
            </Badge>
            <Badge variant="outline">{response.final_reason}</Badge>
            <Badge variant="outline">{response.total_tokens_used} tokens</Badge>
            <Badge variant="outline">threshold {response.confidence_threshold}</Badge>
          </div>

          {/* Answer */}
          {response.answer && (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-medium">Answer</h3>
                <Badge
                  variant={response.answer.can_answer ? 'default' : 'destructive'}
                  className="text-xs"
                >
                  {response.answer.can_answer ? 'can answer' : 'cannot answer'}
                </Badge>
                <Badge variant="outline" className="text-xs">
                  confidence {response.answer.grounded_confidence.toFixed(2)}
                </Badge>
              </div>
              <div className="rounded-md border bg-background p-4">
                <div className="prose prose-sm max-w-none dark:prose-invert">
                  <Markdown>{response.answer.content}</Markdown>
                </div>
              </div>
            </div>
          )}

          {/* Query Plan */}
          <div className="space-y-2">
            <h3 className="text-sm font-medium">Query Plan</h3>
            {response.query_plan.error && (
              <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                {response.query_plan.error}
              </div>
            )}
            {response.query_plan.standalone_query && (
              <div className="rounded-md border bg-muted/30 px-3 py-2 font-mono text-xs">
                {response.query_plan.standalone_query}
              </div>
            )}
            {response.query_plan.clarifying_question && (
              <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">
                {response.query_plan.clarifying_question}
              </div>
            )}
            {searchQueries.length > 0 && (
              <div className="space-y-1">
                {searchQueries.map((q, i) => (
                  <div key={`${q}-${i}`} className="rounded-md border bg-background px-3 py-1.5 font-mono text-xs">
                    {q}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Retrieval */}
          {retrievalResults.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-sm font-medium">
                Retrieval ({response.retrieval.result_count} results)
              </h3>
              {response.retrieval.error && (
                <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                  {response.retrieval.error}
                </div>
              )}
              {retrievalResults.map((r) => (
                <div key={`${r.reference_id}-${r.chunk_index}`} className="rounded-md border p-3 text-sm">
                  <div className="mb-1 flex items-center gap-2">
                    <span className="font-medium">{r.title || 'Untitled'}</span>
                    <Badge variant="outline" className="text-xs">
                      {r.combined_score.toFixed(3)}
                    </Badge>
                    <Badge variant="outline" className="text-xs">
                      {r.source_type}
                    </Badge>
                  </div>
                  {r.url && (
                    <a
                      href={r.url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-xs text-primary hover:underline"
                    >
                      {r.url}
                    </a>
                  )}
                  <p className="mt-1 text-xs text-muted-foreground">{r.snippet}</p>
                </div>
              ))}
            </div>
          )}

          {/* Raw JSON */}
          <details className="text-sm">
            <summary className="cursor-pointer font-medium text-muted-foreground hover:text-foreground">
              Raw Response
            </summary>
            <pre className="mt-2 max-h-[400px] overflow-auto rounded-md border bg-muted/30 p-3 text-xs leading-5">
              {JSON.stringify(response, null, 2)}
            </pre>
          </details>
        </div>
      )}
    </div>
  )
}
