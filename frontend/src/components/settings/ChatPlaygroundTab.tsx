import { useEffect, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import Markdown from 'react-markdown';
import { Bot, FlaskConical, Plus, RotateCcw, Search, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import type {
  MessageSenderType,
  SupportAIPreviewHistoryTurn,
  SupportAIPreviewResponse,
} from '@/lib/pmTypes';
import { useSupportAgents, useChatSettings } from '@/hooks/queries/useSupport';
import { LINEAR_CARD_CLASS } from './settingsConstants';

type EditableHistoryTurn = SupportAIPreviewHistoryTurn & { id: string };

let turnIdCounter = 0;

function createHistoryTurn(senderType: MessageSenderType = 'customer', content = ''): EditableHistoryTurn {
  turnIdCounter += 1;
  return {
    id: `preview-turn-${turnIdCounter}`,
    sender_type: senderType,
    message_type: 'reply',
    content,
  };
}

const SENDER_OPTIONS: Array<{ value: MessageSenderType; label: string }> = [
  { value: 'customer', label: 'Customer' },
  { value: 'ai', label: 'AI' },
  { value: 'agent', label: 'Agent' },
  { value: 'user', label: 'Assistant' },
];

function decisionVariant(decision: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (decision) {
    case 'answer':
      return 'default';
    case 'clarify':
      return 'secondary';
    case 'handoff':
      return 'destructive';
    default:
      return 'outline';
  }
}

function normalizePreviewResponse(data: SupportAIPreviewResponse): SupportAIPreviewResponse {
  const queryPlan = data.query_plan ?? {
    decision: '',
    standalone_query: '',
    search_queries: [],
    clarifying_question: '',
    reason: '',
    tokens_used: 0,
    fallback_used: false,
  };
  const retrieval = data.retrieval ?? {
    query_count: 0,
    result_count: 0,
    results: [],
  };

  return {
    ...data,
    query_plan: {
      ...queryPlan,
      search_queries: queryPlan.search_queries ?? [],
    },
    retrieval: {
      ...retrieval,
      results: retrieval.results ?? [],
    },
    answer: data.answer
      ? {
          ...data.answer,
          source_doc_ids: data.answer.source_doc_ids ?? [],
        }
      : undefined,
  };
}

export function ChatPlaygroundTab({ workspaceId }: { workspaceId: string }) {
  const { data: supportAgents = [], isLoading: agentsLoading } = useSupportAgents(workspaceId);
  const { data: chatSettings } = useChatSettings(workspaceId);

  const configuredAgentId = chatSettings?.settings.ai_agent_id ?? '';
  const [agentId, setAgentId] = useState('');
  const [conversationId, setConversationId] = useState('');
  const [latestMessage, setLatestMessage] = useState('');
  const [includeAnswer, setIncludeAnswer] = useState(true);
  const [maxResults, setMaxResults] = useState(8);
  const [history, setHistory] = useState<EditableHistoryTurn[]>([]);
  const [response, setResponse] = useState<SupportAIPreviewResponse | null>(null);

  useEffect(() => {
    if (agentId || supportAgents.length === 0) {
      return;
    }
    const configuredExists = configuredAgentId && supportAgents.some((agent) => agent.id === configuredAgentId);
    setAgentId(configuredExists ? configuredAgentId : supportAgents[0].id);
  }, [agentId, configuredAgentId, supportAgents]);

  const previewMutation = useMutation({
    mutationFn: async () => {
      if (!agentId) {
        throw new Error('Select a support agent first.');
      }
      if (!latestMessage.trim()) {
        throw new Error('Latest message is required.');
      }

      const normalizedHistory = history
        .map((turn) => ({
          sender_type: turn.sender_type,
          message_type: turn.message_type || 'reply',
          content: turn.content.trim(),
        }))
        .filter((turn) => turn.content.length > 0);

      return unwrap(await agentService.previewSupportReply(workspaceId, agentId, {
        message: latestMessage.trim(),
        conversation_id: conversationId.trim() || undefined,
        history: normalizedHistory.length > 0 ? normalizedHistory : undefined,
        include_answer: includeAnswer,
        max_results: Math.max(1, Math.min(maxResults || 8, 12)),
      }));
    },
    onSuccess: (data) => {
      setResponse(normalizePreviewResponse(data));
    },
    onError: (error: Error) => {
      toast.error('Preview failed', { description: error.message });
    },
  });

  const addTurn = (senderType: MessageSenderType = 'customer') => {
    setHistory((current) => [...current, createHistoryTurn(senderType)]);
  };

  const updateTurn = (id: string, patch: Partial<SupportAIPreviewHistoryTurn>) => {
    setHistory((current) =>
      current.map((turn) => (turn.id === id ? { ...turn, ...patch } : turn)),
    );
  };

  const removeTurn = (id: string) => {
    setHistory((current) => current.filter((turn) => turn.id !== id));
  };

  const loadSample = () => {
    setConversationId('');
    setLatestMessage('features');
    setHistory([
      createHistoryTurn('customer', 'Publer or ContentStudio?'),
      createHistoryTurn('ai', 'ContentStudio is stronger for agencies and richer analytics.'),
    ]);
    setIncludeAnswer(true);
    setMaxResults(8);
    setResponse(null);
  };

  const clearScenario = () => {
    setConversationId('');
    setLatestMessage('');
    setHistory([]);
    setResponse(null);
  };

  const hasManualHistory = history.some((turn) => turn.content.trim().length > 0);
  const searchQueries = response?.query_plan.search_queries ?? [];
  const retrievalResults = response?.retrieval.results ?? [];
  const answerSourceDocIDs = response?.answer?.source_doc_ids ?? [];

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <CardTitle className="flex items-center gap-2 text-base">
                <FlaskConical className="h-4 w-4" />
                Chat Playground
              </CardTitle>
              <CardDescription>
                Dry-run the support AI planner, retrieval, and answer pipeline without touching real conversations.
              </CardDescription>
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={loadSample}>
                Load Sample
              </Button>
              <Button type="button" variant="outline" size="sm" onClick={clearScenario}>
                Clear
              </Button>
              <Button type="button" size="sm" onClick={() => previewMutation.mutate()} disabled={previewMutation.isPending || agentsLoading}>
                {previewMutation.isPending ? 'Running...' : 'Run Preview'}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>Support Agent</Label>
              <Select value={agentId} onValueChange={setAgentId}>
                <SelectTrigger>
                  <SelectValue placeholder={agentsLoading ? 'Loading support agents...' : 'Select a support agent'} />
                </SelectTrigger>
                <SelectContent>
                  {supportAgents.map((agent) => (
                    <SelectItem key={agent.id} value={agent.id}>
                      {agent.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {configuredAgentId && agentId === configuredAgentId
                  ? 'Using the currently configured chat AI agent.'
                  : 'Pick the support agent whose prompt, provider, and knowledge sources you want to test.'}
              </p>
            </div>

            <div className="grid gap-4 md:grid-cols-[1fr_120px]">
              <div className="space-y-2">
                <Label htmlFor="conversationId">Conversation ID</Label>
                <Input
                  id="conversationId"
                  value={conversationId}
                  onChange={(event) => setConversationId(event.target.value)}
                  placeholder="Optional: replay an existing conversation"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="maxResults">Max Results</Label>
                <Input
                  id="maxResults"
                  type="number"
                  min={1}
                  max={12}
                  value={maxResults}
                  onChange={(event) => setMaxResults(Number(event.target.value) || 8)}
                />
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="latestMessage">Latest Message</Label>
            <Textarea
              id="latestMessage"
              value={latestMessage}
              onChange={(event) => setLatestMessage(event.target.value)}
              placeholder="Enter the latest customer message to test"
              rows={3}
            />
          </div>

          <div className="flex items-center justify-between rounded-md border px-4 py-3">
            <div>
              <Label className="text-sm">Generate Final Answer</Label>
              <p className="text-xs text-muted-foreground">
                Disable this to inspect just the planner and retrieval stages.
              </p>
            </div>
            <Switch checked={includeAnswer} onCheckedChange={setIncludeAnswer} />
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <Label className="text-sm">Manual History</Label>
                <p className="text-xs text-muted-foreground">
                  Use this for follow-up testing. If any manual history is present, it overrides the conversation ID above.
                </p>
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" size="sm" onClick={() => addTurn('customer')}>
                  <Plus className="mr-1 h-3.5 w-3.5" />
                  Add Turn
                </Button>
              </div>
            </div>

            {history.length === 0 ? (
              <div className="rounded-md border border-dashed px-4 py-6 text-sm text-muted-foreground">
                No manual history yet. Add turns here, or leave it empty and use a conversation ID or standalone message.
              </div>
            ) : (
              <div className="space-y-3">
                {history.map((turn, index) => (
                  <div key={turn.id} className="rounded-md border p-3">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div className="flex items-center gap-3">
                        <Badge variant="outline">Turn {index + 1}</Badge>
                        <Select
                          value={turn.sender_type}
                          onValueChange={(value) => updateTurn(turn.id, { sender_type: value as MessageSenderType })}
                        >
                          <SelectTrigger className="w-[160px]">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SENDER_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <Button type="button" variant="ghost" size="icon" onClick={() => removeTurn(turn.id)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>

                    <Textarea
                      value={turn.content}
                      onChange={(event) => updateTurn(turn.id, { content: event.target.value })}
                      placeholder="Conversation turn content"
                      rows={3}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {response ? (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
          <div className="space-y-6">
            <Card className={LINEAR_CARD_CLASS}>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Outcome</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-wrap items-center gap-3 text-sm">
                <Badge variant={decisionVariant(response.final_decision)}>{response.final_decision}</Badge>
                <Badge variant="outline">{response.final_reason}</Badge>
                <Badge variant="outline">{response.conversation_source}</Badge>
                <Badge variant="outline">{response.total_tokens_used} tokens</Badge>
                <Badge variant="outline">threshold {response.confidence_threshold}</Badge>
                {hasManualHistory && (
                  <Badge variant="secondary">manual history</Badge>
                )}
              </CardContent>
            </Card>

            <Card className={LINEAR_CARD_CLASS}>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Query Plan</CardTitle>
                <CardDescription>How the planner resolved the latest message before retrieval.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4 text-sm">
                <div className="flex flex-wrap gap-2">
                  <Badge variant={decisionVariant(response.query_plan.decision)}>{response.query_plan.decision}</Badge>
                  <Badge variant="outline">{response.query_plan.reason}</Badge>
                  <Badge variant="outline">{response.query_plan.tokens_used} tokens</Badge>
                  {response.query_plan.fallback_used && (
                    <Badge variant="destructive">fallback</Badge>
                  )}
                </div>

                {response.query_plan.error && (
                  <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {response.query_plan.error}
                  </div>
                )}

                {response.query_plan.standalone_query && (
                  <div className="space-y-1">
                    <Label className="text-xs uppercase tracking-wide text-muted-foreground">Standalone Query</Label>
                    <div className="rounded-md border bg-muted/30 px-3 py-2 font-mono text-xs">
                      {response.query_plan.standalone_query}
                    </div>
                  </div>
                )}

                {response.query_plan.clarifying_question && (
                  <div className="space-y-1">
                    <Label className="text-xs uppercase tracking-wide text-muted-foreground">Clarifying Question</Label>
                    <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">
                      {response.query_plan.clarifying_question}
                    </div>
                  </div>
                )}

                <div className="space-y-2">
                  <Label className="text-xs uppercase tracking-wide text-muted-foreground">Search Queries</Label>
                  {searchQueries.length === 0 ? (
                    <div className="rounded-md border border-dashed px-3 py-4 text-xs text-muted-foreground">
                      No retrieval queries were generated for this decision.
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {searchQueries.map((query, index) => (
                        <div key={`${query}-${index}`} className="rounded-md border bg-background px-3 py-2 font-mono text-xs">
                          {query}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>

            <Card className={LINEAR_CARD_CLASS}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-base">Retrieval</CardTitle>
                    <CardDescription>Chunks selected from the configured knowledge sources.</CardDescription>
                  </div>
                  <div className="flex gap-2 text-xs">
                    <Badge variant="outline">{response.retrieval.query_count} queries</Badge>
                    <Badge variant="outline">{response.retrieval.result_count} results</Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {response.retrieval.error && (
                  <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {response.retrieval.error}
                  </div>
                )}

                {retrievalResults.length === 0 ? (
                  <div className="rounded-md border border-dashed px-4 py-6 text-sm text-muted-foreground">
                    No retrieval hits for this preview.
                  </div>
                ) : (
                  retrievalResults.map((result) => (
                    <div key={`${result.reference_id}-${result.chunk_index}`} className="rounded-md border p-3">
                      <div className="mb-2 flex flex-wrap items-center gap-2">
                        <Badge variant="outline">{result.reference_id}</Badge>
                        <Badge variant="outline">{result.source_type}</Badge>
                        <Badge variant="outline">score {result.combined_score.toFixed(3)}</Badge>
                        <Badge variant="outline">vec {result.vector_score.toFixed(3)}</Badge>
                        <Badge variant="outline">lex {result.lexical_score.toFixed(3)}</Badge>
                      </div>
                      <div className="space-y-1">
                        <p className="font-medium">{result.title || 'Untitled chunk'}</p>
                        {result.url && (
                          <a href={result.url} target="_blank" rel="noreferrer" className="block text-xs text-primary hover:underline">
                            {result.url}
                          </a>
                        )}
                        <p className="text-sm text-muted-foreground">{result.snippet}</p>
                      </div>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
          </div>

          <div className="space-y-6">
            <Card className={LINEAR_CARD_CLASS}>
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Bot className="h-4 w-4" />
                  Answer Preview
                </CardTitle>
                <CardDescription>Dry-run answer using the same support response contract as production.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {response.answer ? (
                  <>
                    <div className="flex flex-wrap gap-2 text-xs">
                      <Badge variant={response.answer.can_answer ? 'default' : 'destructive'}>
                        can_answer: {String(response.answer.can_answer)}
                      </Badge>
                      <Badge variant="outline">{response.answer.provider}</Badge>
                      <Badge variant="outline">{response.answer.model}</Badge>
                      <Badge variant="outline">llm {response.answer.llm_confidence.toFixed(2)}</Badge>
                      <Badge variant="outline">grounded {response.answer.grounded_confidence.toFixed(2)}</Badge>
                      <Badge variant="outline">{response.answer.tokens_used} tokens</Badge>
                    </div>

                    {answerSourceDocIDs.length > 0 && (
                      <div className="space-y-2">
                        <Label className="text-xs uppercase tracking-wide text-muted-foreground">Cited Sources</Label>
                        <div className="flex flex-wrap gap-2">
                          {answerSourceDocIDs.map((docId) => (
                            <Badge key={docId} variant="outline">{docId}</Badge>
                          ))}
                        </div>
                      </div>
                    )}

                    <div className="rounded-md border bg-background p-3">
                      <div className="prose prose-sm max-w-none dark:prose-invert">
                        <Markdown>{response.answer.content}</Markdown>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="rounded-md border border-dashed px-4 py-6 text-sm text-muted-foreground">
                    No answer was generated for this preview. This happens when the planner decided to clarify or hand off, or when answer generation was disabled.
                  </div>
                )}
              </CardContent>
            </Card>

            <Card className={LINEAR_CARD_CLASS}>
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Search className="h-4 w-4" />
                  Raw Response
                </CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="max-h-[560px] overflow-auto rounded-md border bg-muted/30 p-3 text-xs leading-5">
                  {JSON.stringify(response, null, 2)}
                </pre>
              </CardContent>
            </Card>
          </div>
        </div>
      ) : (
        <Card className={LINEAR_CARD_CLASS}>
          <CardContent className="flex items-center justify-between gap-4 py-6">
            <div className="space-y-1">
              <p className="font-medium">Run a preview to inspect planner, retrieval, and answer behavior.</p>
              <p className="text-sm text-muted-foreground">
                Use manual history for follow-up fragments like “features” or “pricing”. Use conversation replay when you want to debug a real thread by ID.
              </p>
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={loadSample}>
                <RotateCcw className="mr-1 h-3.5 w-3.5" />
                Sample Thread
              </Button>
              <Button type="button" onClick={() => previewMutation.mutate()} disabled={previewMutation.isPending || !latestMessage.trim() || !agentId}>
                {previewMutation.isPending ? 'Running...' : 'Run Preview'}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
