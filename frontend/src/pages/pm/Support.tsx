import { useCallback, useEffect, useRef, useState } from 'react';
import { MessageSquare, Plus, Send, ArrowLeft, Bot, Loader2, ShieldCheck } from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { supportService } from '@/lib/services/supportService';
import { agentService } from '@/lib/services/agentService';
import type {
  SupportTicket,
  SupportMessage,
  TicketStatus,
  TicketPriority,
  CreateTicketRequest,
  Agent,
  AgentRun,
  AgentRunArtifact,
} from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Card } from '@/components/ui/card';

// ── Badge colors ────────────────────────────────────────────────────

const STATUS_COLORS: Record<TicketStatus, string> = {
  open: 'bg-blue-100 text-blue-700',
  in_progress: 'bg-amber-100 text-amber-700',
  waiting: 'bg-purple-100 text-purple-700',
  resolved: 'bg-green-100 text-green-700',
  closed: 'bg-gray-100 text-gray-600',
};

const STATUS_LABELS: Record<TicketStatus, string> = {
  open: 'Open',
  in_progress: 'In Progress',
  waiting: 'Waiting',
  resolved: 'Resolved',
  closed: 'Closed',
};

const PRIORITY_COLORS: Record<TicketPriority, string> = {
  low: 'bg-gray-100 text-gray-600',
  medium: 'bg-blue-100 text-blue-700',
  high: 'bg-amber-100 text-amber-700',
  urgent: 'bg-red-100 text-red-700',
};

// ── Helpers ─────────────────────────────────────────────────────────

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

// ── Ticket List Item ────────────────────────────────────────────────

function TicketRow({
  ticket,
  isSelected,
  onSelect,
}: {
  ticket: SupportTicket;
  isSelected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left border-b px-3 py-2.5 transition-colors hover:bg-muted/50 ${
        isSelected ? 'bg-muted' : ''
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">#{ticket.display_id}</span>
            <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 ${STATUS_COLORS[ticket.status]}`}>
              {STATUS_LABELS[ticket.status]}
            </Badge>
            <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 ${PRIORITY_COLORS[ticket.priority]}`}>
              {ticket.priority}
            </Badge>
          </div>
          <p className="mt-0.5 truncate text-sm font-medium">{ticket.subject}</p>
          {ticket.customer_name && (
            <p className="truncate text-xs text-muted-foreground">{ticket.customer_name}</p>
          )}
        </div>
        <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(ticket.updated_at)}</span>
      </div>
    </button>
  );
}

// ── Message Bubble ──────────────────────────────────────────────────

function MessageBubble({ message }: { message: SupportMessage }) {
  const isCustomer = message.sender_type === 'customer';
  const isInternal = message.is_internal;

  return (
    <div className={`flex ${isCustomer ? 'justify-start' : 'justify-end'}`}>
      <div
        className={`max-w-[75%] rounded-lg px-3 py-2 text-sm ${
          isInternal
            ? 'border-2 border-dashed border-amber-300 bg-amber-50 text-amber-900'
            : isCustomer
              ? 'bg-muted text-foreground'
              : 'bg-primary text-primary-foreground'
        }`}
      >
        <div className="mb-1 flex items-center gap-2">
          <span className="text-[11px] font-medium">
            {message.sender_display_name ?? message.sender_type}
          </span>
          {isInternal && <span className="text-[10px] italic">(internal note)</span>}
          <span className="text-[10px] opacity-70">{timeAgo(message.created_at)}</span>
        </div>
        <p className="whitespace-pre-wrap">{message.content}</p>
      </div>
    </div>
  );
}

// ── Main Page ───────────────────────────────────────────────────────

export function SupportPage() {
  useTitle('Support');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  // ── Tickets state ──
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>('open');

  // ── Selected ticket state ──
  const [selectedTicket, setSelectedTicket] = useState<SupportTicket | null>(null);
  const [messages, setMessages] = useState<SupportMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [runArtifacts, setRunArtifacts] = useState<AgentRunArtifact[]>([]);
  const [runningAgent, setRunningAgent] = useState(false);
  const [approvingRun, setApprovingRun] = useState<string | null>(null);
  const [supportAgents, setSupportAgents] = useState<Agent[]>([]);

  // ── Reply state ──
  const [replyContent, setReplyContent] = useState('');
  const [isInternal, setIsInternal] = useState(false);
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // ── Create dialog ──
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState<CreateTicketRequest>({
    subject: '',
    priority: 'medium',
    customer_name: '',
    customer_email: '',
  });
  const [creating, setCreating] = useState(false);

  // ── Actions state ──
  const [actionStatus, setActionStatus] = useState<TicketStatus | ''>('');
  const [linkStoryId, setLinkStoryId] = useState('');
  const [assignAgentId, setAssignAgentId] = useState('');

  // ── Mobile view state ──
  const [mobileShowThread, setMobileShowThread] = useState(false);

  // ── Load tickets ──
  const loadTickets = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const filters = statusFilter !== 'all' ? { status: statusFilter } : undefined;
    const res = await supportService.listTickets(workspaceId, filters);
    if (res.error) {
      setError(res.error);
    } else {
      const data = res.data;
      setTickets(Array.isArray(data) ? data : (data as any)?.data ?? []);
    }
    setLoading(false);
  }, [workspaceId, statusFilter]);

  const loadSupportAgents = useCallback(async () => {
    if (!workspaceId) return;
    const res = await agentService.list(workspaceId);
    if (res.error) return;
    setSupportAgents((res.data ?? []).filter((agent) => agent.agent_kind === 'llm' && agent.agent_class === 'support'));
  }, [workspaceId]);

  useEffect(() => {
    loadTickets();
  }, [loadTickets]);

  useEffect(() => {
    loadSupportAgents();
  }, [loadSupportAgents]);

  // ── Load messages ──
  const loadMessages = useCallback(async () => {
    if (!workspaceId || !selectedTicket) return;
    setMessagesLoading(true);
    const res = await supportService.listMessages(workspaceId, selectedTicket.id);
    if (!res.error) {
      setMessages(res.data ?? []);
    }
    setMessagesLoading(false);
  }, [workspaceId, selectedTicket]);

  const loadAgentRuns = useCallback(async () => {
    if (!workspaceId || !selectedTicket?.assigned_agent_id) {
      setAgentRuns([]);
      setSelectedRunId(null);
      setRunArtifacts([]);
      return;
    }
    const res = await agentService.listRuns(workspaceId, selectedTicket.assigned_agent_id);
    if (res.error) return;
    const runs = (res.data?.data ?? []).filter(
      (run) => run.target_type === 'support_ticket' && run.target_id === selectedTicket.id
    );
    setAgentRuns(runs);
    if (runs[0]) {
      setSelectedRunId(runs[0].id);
      const artifactsRes = await agentService.listRunArtifacts(workspaceId, runs[0].id);
      if (!artifactsRes.error) {
        setRunArtifacts(artifactsRes.data ?? []);
      }
    } else {
      setSelectedRunId(null);
      setRunArtifacts([]);
    }
  }, [workspaceId, selectedTicket]);

  useEffect(() => {
    if (selectedTicket) {
      loadMessages();
      loadAgentRuns();
      setReplyContent('');
      setIsInternal(false);
    } else {
      setMessages([]);
      setAgentRuns([]);
      setSelectedRunId(null);
      setRunArtifacts([]);
    }
  }, [selectedTicket, loadMessages, loadAgentRuns]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // ── Send reply ──
  const handleSendReply = async () => {
    if (!workspaceId || !selectedTicket || !replyContent.trim()) return;
    setSending(true);
    const res = await supportService.createMessage(workspaceId, selectedTicket.id, {
      content: replyContent.trim(),
      is_internal: isInternal,
    });
    if (!res.error) {
      setReplyContent('');
      setIsInternal(false);
      loadMessages();
    }
    setSending(false);
  };

  // ── Create ticket ──
  const handleCreate = async () => {
    if (!workspaceId || !createForm.subject.trim()) return;
    setCreating(true);
    const res = await supportService.createTicket(workspaceId, createForm);
    if (!res.error) {
      setCreateOpen(false);
      setCreateForm({ subject: '', priority: 'medium', customer_name: '', customer_email: '' });
      loadTickets();
    }
    setCreating(false);
  };

  // ── Ticket actions ──
  const handleChangeStatus = async (status: TicketStatus) => {
    if (!workspaceId || !selectedTicket) return;
    setActionStatus('');
    const res = await supportService.updateStatus(workspaceId, selectedTicket.id, status);
    if (!res.error && res.data) {
      setSelectedTicket(res.data);
      loadTickets();
    }
  };

  const handleLinkStory = async () => {
    if (!workspaceId || !selectedTicket || !linkStoryId.trim()) return;
    await supportService.linkStory(workspaceId, selectedTicket.id, { story_id: linkStoryId.trim() });
    setLinkStoryId('');
    setSelectedTicket({ ...selectedTicket, linked_story_id: linkStoryId.trim() });
  };

  const handleAssignAgent = async () => {
    if (!workspaceId || !selectedTicket || !assignAgentId.trim()) return;
    await supportService.assignAgent(workspaceId, selectedTicket.id, { agent_id: assignAgentId.trim() });
    setAssignAgentId('');
    setSelectedTicket({ ...selectedTicket, assigned_agent_id: assignAgentId.trim() });
  };

  const handleRunAgent = async () => {
    if (!workspaceId || !selectedTicket) return;
    setRunningAgent(true);
    try {
      await supportService.runAgent(workspaceId, selectedTicket.id);
      await loadAgentRuns();
    } finally {
      setRunningAgent(false);
    }
  };

  const handleApproveRun = async (runId: string) => {
    if (!workspaceId) return;
    setApprovingRun(runId);
    try {
      await agentService.approveRun(workspaceId, runId, { send_message: true });
      await loadAgentRuns();
      await loadMessages();
    } finally {
      setApprovingRun(null);
    }
  };

  const selectTicket = (ticket: SupportTicket) => {
    setSelectedTicket(ticket);
    setMobileShowThread(true);
  };

  const assignedAgent = selectedTicket?.assigned_agent_id
    ? supportAgents.find((agent) => agent.id === selectedTicket.assigned_agent_id)
    : null;

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  // ── Render ──
  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-2">
        <h1 className="text-xl font-semibold">Support</h1>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-1.5 h-4 w-4" />
          New Ticket
        </Button>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* Left panel: ticket list */}
        <div
          className={`flex w-full flex-col border-r md:w-80 lg:w-96 ${
            mobileShowThread ? 'hidden md:flex' : 'flex'
          }`}
        >
          <div className="border-b px-2 py-1.5">
            <Tabs value={statusFilter} onValueChange={setStatusFilter}>
              <TabsList className="h-8 w-full">
                <TabsTrigger value="open" className="text-xs">Open</TabsTrigger>
                <TabsTrigger value="in_progress" className="text-xs">In Progress</TabsTrigger>
                <TabsTrigger value="waiting" className="text-xs">Waiting</TabsTrigger>
                <TabsTrigger value="resolved" className="text-xs">Resolved</TabsTrigger>
                <TabsTrigger value="all" className="text-xs">All</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          <div className="flex-1 overflow-y-auto">
            {loading && <p className="p-3 text-sm text-muted-foreground">Loading...</p>}
            {error && <p className="p-3 text-sm text-destructive">{error}</p>}
            {!loading && tickets.length === 0 && !error && (
              <div className="flex flex-col items-center justify-center gap-3 py-16">
                <MessageSquare className="h-10 w-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">No tickets found.</p>
              </div>
            )}
            {tickets.map((ticket) => (
              <TicketRow
                key={ticket.id}
                ticket={ticket}
                isSelected={selectedTicket?.id === ticket.id}
                onSelect={() => selectTicket(ticket)}
              />
            ))}
          </div>
        </div>

        {/* Right panel: ticket detail + thread */}
        <div
          className={`flex min-w-0 flex-1 flex-col ${
            !mobileShowThread ? 'hidden md:flex' : 'flex'
          }`}
        >
          {!selectedTicket ? (
            <div className="flex flex-1 items-center justify-center">
              <p className="text-sm text-muted-foreground">Select a ticket to view</p>
            </div>
          ) : (
            <>
              {/* Ticket header */}
              <div className="border-b px-4 py-3">
                <div className="flex items-center gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="md:hidden h-8 w-8 p-0"
                    onClick={() => setMobileShowThread(false)}
                  >
                    <ArrowLeft className="h-4 w-4" />
                  </Button>
                  <span className="text-xs text-muted-foreground">#{selectedTicket.display_id}</span>
                  <Badge variant="secondary" className={`${STATUS_COLORS[selectedTicket.status]}`}>
                    {STATUS_LABELS[selectedTicket.status]}
                  </Badge>
                  <Badge variant="secondary" className={`${PRIORITY_COLORS[selectedTicket.priority]}`}>
                    {selectedTicket.priority}
                  </Badge>
                </div>
                <h2 className="mt-1 text-lg font-semibold">{selectedTicket.subject}</h2>
                <div className="mt-1 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                  {selectedTicket.customer_name && <span>Customer: {selectedTicket.customer_name}</span>}
                  {selectedTicket.assigned_agent_id && (
                    <span>Agent: {assignedAgent?.name ?? selectedTicket.assigned_agent_id.slice(0, 8)}</span>
                  )}
                  {selectedTicket.linked_story_id && (
                    <span>Story: {selectedTicket.linked_story_id.slice(0, 8)}...</span>
                  )}
                </div>

                {/* Actions row */}
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <Select
                    value={actionStatus || selectedTicket.status}
                    onValueChange={(v) => handleChangeStatus(v as TicketStatus)}
                  >
                    <SelectTrigger className="h-7 w-32 text-xs">
                      <SelectValue placeholder="Status" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="open">Open</SelectItem>
                      <SelectItem value="in_progress">In Progress</SelectItem>
                      <SelectItem value="waiting">Waiting</SelectItem>
                      <SelectItem value="resolved">Resolved</SelectItem>
                      <SelectItem value="closed">Closed</SelectItem>
                    </SelectContent>
                  </Select>

                  <div className="flex items-center gap-1">
                    <Select value={assignAgentId} onValueChange={setAssignAgentId} disabled={supportAgents.length === 0}>
                      <SelectTrigger className="h-7 w-40 text-xs">
                        <SelectValue placeholder="Support agent" />
                      </SelectTrigger>
                      <SelectContent>
                        {supportAgents.map((agent) => (
                          <SelectItem key={agent.id} value={agent.id}>
                            {agent.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 text-xs"
                      disabled={!assignAgentId}
                      onClick={handleAssignAgent}
                    >
                      Assign
                    </Button>
                  </div>

                  <div className="flex items-center gap-1">
                    <Input
                      className="h-7 w-28 text-xs"
                      placeholder="Story ID"
                      value={linkStoryId}
                      onChange={(e) => setLinkStoryId(e.target.value)}
                    />
                    <Button size="sm" variant="outline" className="h-7 text-xs" onClick={handleLinkStory}>
                      Link
                    </Button>
                  </div>

                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 gap-1 text-xs"
                    disabled={runningAgent || !selectedTicket.assigned_agent_id}
                    onClick={handleRunAgent}
                  >
                    {runningAgent ? <Loader2 className="h-3 w-3 animate-spin" /> : <Bot className="h-3 w-3" />}
                    Run Agent
                  </Button>
                </div>

                {agentRuns.length > 0 && (
                  <Card className="mt-3 space-y-2 p-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Bot className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm font-medium">Agent Runs</span>
                      </div>
                      {agentRuns[0]?.approval_state === 'pending' && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 gap-1 text-xs"
                          disabled={approvingRun === agentRuns[0].id}
                          onClick={() => handleApproveRun(agentRuns[0].id)}
                        >
                          {approvingRun === agentRuns[0].id ? <Loader2 className="h-3 w-3 animate-spin" /> : <ShieldCheck className="h-3 w-3" />}
                          Approve Draft
                        </Button>
                      )}
                    </div>
                    <div className="space-y-2">
                      {agentRuns.map((run) => (
                        <button
                          key={run.id}
                          type="button"
                          className={`w-full rounded border px-2 py-2 text-left text-xs ${selectedRunId === run.id ? 'border-primary bg-muted' : 'border-border/60'}`}
                          onClick={async () => {
                            setSelectedRunId(run.id);
                            const artifactsRes = await agentService.listRunArtifacts(workspaceId!, run.id);
                            if (!artifactsRes.error) setRunArtifacts(artifactsRes.data ?? []);
                          }}
                        >
                          <div className="flex items-center justify-between">
                            <span>{run.status}</span>
                            <span className="text-muted-foreground">{run.approval_state}</span>
                          </div>
                          <div className="mt-1 text-[10px] text-muted-foreground">{timeAgo(run.created_at)} · {run.runtime_kind}</div>
                        </button>
                      ))}
                    </div>
                    {runArtifacts.length > 0 && (
                      <div className="space-y-2 border-t pt-2">
                        {runArtifacts.map((artifact) => (
                          <div key={artifact.id} className="rounded border border-border/60 bg-muted/40 p-2">
                            <div className="text-[11px] font-medium">
                              {artifact.artifact_type} <span className="text-muted-foreground">({artifact.format})</span>
                            </div>
                            {artifact.inline_content && (
                              <pre className="mt-1 max-h-28 overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground">
                                {artifact.inline_content.slice(0, 1200)}
                                {artifact.inline_content.length > 1200 && '...'}
                              </pre>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </Card>
                )}
              </div>

              {/* Messages thread */}
              <div className="flex-1 overflow-y-auto p-4 space-y-3">
                {messagesLoading && <p className="text-sm text-muted-foreground">Loading messages...</p>}
                {messages.map((msg) => (
                  <MessageBubble key={msg.id} message={msg} />
                ))}
                <div ref={messagesEndRef} />
              </div>

              {/* Reply box */}
              <div className="border-t p-3">
                <div className="flex items-center gap-2 mb-2">
                  <label className="flex items-center gap-1.5 text-xs text-muted-foreground cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isInternal}
                      onChange={(e) => setIsInternal(e.target.checked)}
                      className="rounded"
                    />
                    Internal note
                  </label>
                </div>
                <div className="flex gap-2">
                  <Textarea
                    className="min-h-[60px] flex-1 text-sm"
                    placeholder={isInternal ? 'Write an internal note...' : 'Type your reply...'}
                    value={replyContent}
                    onChange={(e) => setReplyContent(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                        handleSendReply();
                      }
                    }}
                  />
                  <Button
                    size="sm"
                    className="h-auto self-end"
                    disabled={sending || !replyContent.trim()}
                    onClick={handleSendReply}
                  >
                    <Send className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Create ticket dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create Ticket</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="ticket-subject">Subject</Label>
              <Input
                id="ticket-subject"
                value={createForm.subject}
                onChange={(e) => setCreateForm((f) => ({ ...f, subject: e.target.value }))}
                placeholder="Ticket subject"
              />
            </div>
            <div className="space-y-1.5">
              <Label>Priority</Label>
              <Select
                value={createForm.priority ?? 'medium'}
                onValueChange={(v) => setCreateForm((f) => ({ ...f, priority: v as TicketPriority }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Low</SelectItem>
                  <SelectItem value="medium">Medium</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                  <SelectItem value="urgent">Urgent</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ticket-customer-name">Customer Name</Label>
              <Input
                id="ticket-customer-name"
                value={createForm.customer_name ?? ''}
                onChange={(e) => setCreateForm((f) => ({ ...f, customer_name: e.target.value }))}
                placeholder="Customer name"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ticket-customer-email">Customer Email</Label>
              <Input
                id="ticket-customer-email"
                type="email"
                value={createForm.customer_email ?? ''}
                onChange={(e) => setCreateForm((f) => ({ ...f, customer_email: e.target.value }))}
                placeholder="customer@example.com"
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" size="sm" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={creating || !createForm.subject.trim()}
                onClick={handleCreate}
              >
                {creating ? 'Creating...' : 'Create'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
