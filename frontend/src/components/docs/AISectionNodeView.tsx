import { ChangeSet, simplifyChanges } from '@tiptap/pm/changeset';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import { Transform } from '@tiptap/pm/transform';
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { AiMagicIcon, Tick01Icon, AlertCircleIcon, SourceCodeIcon } from '@/lib/icons';
import { useAgents } from '@/hooks/queries/useAgents';
import { docsService } from '@/lib/services/docsService';
import type { DocsAISectionCandidate } from '@/lib/docsTypes';
import type { CitationSourceRef } from './CitationBlockExtension';

export type AISectionStatus = 'draft' | 'generated' | 'needs_review' | 'approved' | 'error';

const STATUS_LABELS: Record<AISectionStatus, string> = {
  draft: 'Draft',
  generated: 'Generated',
  needs_review: 'Needs review',
  approved: 'Approved',
  error: 'Error',
};

const STATUS_STYLES: Record<AISectionStatus, string> = {
  draft: 'border-border bg-muted text-muted-foreground',
  generated: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-300',
  needs_review: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300',
  approved: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300',
  error: 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300',
};

type AISectionDiffChunk = {
  id: string;
  kind: 'added' | 'removed' | 'changed' | 'unchanged';
  before?: string;
  after?: string;
};

function candidateSources(candidate: DocsAISectionCandidate | null): CitationSourceRef[] {
  const raw = candidate?.source_refs;
  if (!raw || typeof raw !== 'object') return [];
  const items = (raw as { items?: unknown }).items;
  return Array.isArray(items) ? (items as CitationSourceRef[]) : [];
}

function extractPlainText(value: unknown): string {
  if (!value) return '';
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) {
    return value.map(extractPlainText).filter(Boolean).join('\n');
  }
  if (typeof value !== 'object') return '';
  const record = value as Record<string, unknown>;
  const ownText = typeof record.text === 'string' ? record.text : '';
  const childText = extractPlainText(record.content);
  return [ownText, childText].filter(Boolean).join(ownText && childText ? '' : '\n').trim();
}

function safeTextBetween(node: ProseMirrorNode, from: number, to: number): string {
  const size = node.content.size;
  const start = Math.max(0, Math.min(from, size));
  const end = Math.max(start, Math.min(to, size));
  return node.textBetween(start, end, '\n', '\n').trim();
}

function fallbackAISectionDiff(currentContent: unknown, candidate: DocsAISectionCandidate | null): AISectionDiffChunk[] {
  const before = extractPlainText(currentContent || candidate?.current_content).trim();
  const after = (candidate?.candidate_text || extractPlainText(candidate?.candidate_content)).trim();
  if (!before && !after) {
    return [{ id: 'empty', kind: 'unchanged', before: 'No visible text changes.' }];
  }
  if (before === after) {
    return [{ id: 'same', kind: 'unchanged', before: before || 'No visible text changes.' }];
  }
  return [{ id: 'fallback', kind: before && after ? 'changed' : before ? 'removed' : 'added', before, after }];
}

function buildAISectionDiff(editor: NodeViewProps['editor'], currentContent: unknown, candidate: DocsAISectionCandidate | null): AISectionDiffChunk[] {
  if (!candidate?.candidate_content) return [];
  try {
    const before = editor.schema.nodeFromJSON((candidate.current_content || currentContent) as Record<string, unknown>);
    const after = editor.schema.nodeFromJSON(candidate.candidate_content as Record<string, unknown>);
    const transform = new Transform(before);
    transform.replaceWith(0, before.content.size, after.content);
    const changes = simplifyChanges(
      ChangeSet.create(before).addSteps(transform.doc, transform.mapping.maps, { source: 'ai_section_candidate' }).changes,
      transform.doc,
    );
    if (changes.length === 0) {
      return [{ id: 'same', kind: 'unchanged', before: 'No visible text changes.' }];
    }
    return changes.map((change, index) => {
      const beforeText = safeTextBetween(before, change.fromA, change.toA);
      const afterText = safeTextBetween(transform.doc, change.fromB, change.toB);
      return {
        id: `${change.fromA}:${change.toA}:${change.fromB}:${change.toB}:${index}`,
        kind: beforeText && afterText ? 'changed' : beforeText ? 'removed' : 'added',
        before: beforeText,
        after: afterText,
      };
    });
  } catch {
    return fallbackAISectionDiff(currentContent, candidate);
  }
}

function sourceCanShowExcerpt(source: CitationSourceRef): boolean {
  return source.access !== 'redacted' && Boolean(source.excerpt);
}

function normalizedStatus(value: unknown): AISectionStatus {
  if (value === 'generated' || value === 'needs_review' || value === 'approved' || value === 'error') {
    return value;
  }
  return 'draft';
}

function formatDate(value: unknown) {
  if (typeof value !== 'string' || !value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export function AISectionNodeView({ node, updateAttributes, editor, getPos }: NodeViewProps) {
  const editable = editor.isEditable;
  const workspaceId = editor.extensionManager.extensions.find((ext) => ext.name === 'aiSection')?.options.workspaceId as string | undefined;
  const documentId = editor.extensionManager.extensions.find((ext) => ext.name === 'aiSection')?.options.documentId as string | undefined;
  const blockId = typeof node.attrs.blockId === 'string' ? node.attrs.blockId : '';
  const status = normalizedStatus(node.attrs.status);
  const title = typeof node.attrs.title === 'string' && node.attrs.title.trim()
    ? node.attrs.title.trim()
    : 'AI section';
  const ownerAgentName = typeof node.attrs.ownerAgentName === 'string' ? node.attrs.ownerAgentName.trim() : '';
  const model = typeof node.attrs.model === 'string' ? node.attrs.model.trim() : '';
  const sourceCount = Number(node.attrs.sourceCount || 0);
  const generatedAt = formatDate(node.attrs.lastGeneratedAt);
  const [focused, setFocused] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(title);
  const [instructions, setInstructions] = useState('');
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [candidate, setCandidate] = useState<DocsAISectionCandidate | null>(null);
  const [pendingRunId, setPendingRunId] = useState('');
  const [busy, setBusy] = useState<'regenerate' | 'approve' | 'reject' | null>(null);
  const wrapperRef = useRef<HTMLElement>(null);
  const { data: agents = [] } = useAgents(workspaceId ?? '');
  const docAgents = agents.filter((agent) => (agent.allowed_targets ?? []).includes('document'));
  const diffChunks = useMemo(
    () => buildAISectionDiff(editor, node.toJSON(), candidate),
    [candidate, editor, node],
  );
  const displayedStatus: AISectionStatus = candidate ? 'needs_review' : status;
  const displayedSourceCount = candidate ? candidateSources(candidate).length : sourceCount;
  const displayedGeneratedAt = candidate
    ? formatDate((candidate.source_refs as { generated_at?: unknown } | undefined)?.generated_at)
    : generatedAt;
  const trimmedInstructions = instructions.trim();

  useEffect(() => {
    setTitleDraft(title);
  }, [title]);

  useEffect(() => {
    if (!selectedAgentId && docAgents.length > 0) {
      setSelectedAgentId(docAgents[0].id);
    }
  }, [docAgents, selectedAgentId]);

  useEffect(() => {
    if (!editable || !workspaceId || !documentId || !blockId) return;
    let cancelled = false;
    docsService.getAISectionCandidate(workspaceId, documentId, blockId).then(({ data }) => {
      if (!cancelled) {
        setCandidate(data?.candidate ?? null);
        if (data?.candidate) setPendingRunId('');
      }
    });
    return () => {
      cancelled = true;
    };
  }, [blockId, documentId, editable, workspaceId]);

  useEffect(() => {
    if (!editable || !workspaceId || !documentId || !blockId || !pendingRunId || candidate) return;
    let cancelled = false;
    const poll = async () => {
      const { data } = await docsService.getAISectionCandidate(workspaceId, documentId, blockId);
      if (cancelled) return;
      if (data?.candidate) {
        setCandidate(data.candidate);
        setPendingRunId('');
      }
    };
    const interval = window.setInterval(poll, 3000);
    void poll();
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [blockId, candidate, documentId, editable, pendingRunId, workspaceId]);

  useEffect(() => {
    const update = () => {
      const pos = typeof getPos === 'function' ? getPos() : undefined;
      if (pos === undefined || pos === null) {
        setFocused(false);
        return;
      }
      try {
        const { from } = editor.state.selection;
        const nodeEnd = pos + node.nodeSize;
        const nextFocused = from > pos && from < nodeEnd;
        setFocused((prev) => (prev === nextFocused ? prev : nextFocused));
      } catch {
        setFocused(false);
      }
    };
    editor.on('selectionUpdate', update);
    return () => {
      editor.off('selectionUpdate', update);
    };
  }, [editor, getPos, node.nodeSize]);

  useEffect(() => {
    if (!focused) return;
    const handleClick = (event: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setFocused(false);
        setEditingTitle(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [focused]);

  const saveTitle = () => {
    const nextTitle = titleDraft.trim() || 'AI section';
    setTitleDraft(nextTitle);
    updateAttributes({ title: nextTitle });
    setEditingTitle(false);
  };

  const regenerate = async () => {
    if (!workspaceId || !documentId || !blockId || !selectedAgentId) return;
    if (!trimmedInstructions) {
      setFocused(true);
      toast.error('Add regeneration instructions first');
      return;
    }
    setBusy('regenerate');
    const { data, error } = await docsService.regenerateAISection(workspaceId, documentId, blockId, {
      agent_id: selectedAgentId,
      instructions: trimmedInstructions,
    });
    setBusy(null);
    if (error) {
      toast.error(error || 'Failed to regenerate section');
      return;
    }
    if (data?.candidate) {
      setCandidate(data.candidate);
      setPendingRunId('');
      toast.success('AI section candidate generated');
      return;
    }
    if (data?.agent_run?.id) {
      setCandidate(null);
      setPendingRunId(data.agent_run.id);
      toast.success('AI section agent started');
      return;
    }
    toast.error('Failed to start AI section agent');
  };

  const approve = async () => {
    if (!workspaceId || !documentId || !blockId || !candidate) return;
    setBusy('approve');
    const { data, error } = await docsService.approveAISection(workspaceId, documentId, blockId);
    setBusy(null);
    if (error) {
      toast.error(error);
      return;
    }
    try {
      const pos = typeof getPos === 'function' ? getPos() : undefined;
      const approvedContent = data?.candidate?.candidate_content ?? candidate.candidate_content;
      if (pos !== undefined && approvedContent) {
        const replacement = editor.schema.nodeFromJSON(approvedContent as Record<string, unknown>);
        editor.view.dispatch(editor.state.tr.replaceWith(pos, pos + node.nodeSize, replacement));
      } else if (data?.content?.content) {
        editor.commands.setContent(data.content.content);
      }
    } catch {
      if (data?.content?.content) editor.commands.setContent(data.content.content);
    }
    setCandidate(null);
    setPendingRunId('');
    toast.success('AI section approved');
  };

  const reject = async () => {
    if (!workspaceId || !documentId || !blockId) return;
    setBusy('reject');
    const { error } = await docsService.rejectAISection(workspaceId, documentId, blockId);
    setBusy(null);
    if (error) {
      toast.error(error);
      return;
    }
    setCandidate(null);
    setPendingRunId('');
    toast.success('AI section candidate rejected');
  };
  const sources = candidateSources(candidate);

  if (!editable) {
    return (
      <NodeViewWrapper>
        <NodeViewContent />
      </NodeViewWrapper>
    );
  }

  return (
    <NodeViewWrapper>
      <section
        ref={wrapperRef}
        className={`docs-ai-section group/ai-section my-5 overflow-hidden rounded-md border border-border bg-background shadow-sm transition-shadow ${
          focused ? 'ring-2 ring-primary/30' : ''
        }`}
        data-ai-section=""
        data-ai-section-status={displayedStatus}
      >
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-muted/35 px-3 py-2" contentEditable={false}>
          <AiMagicIcon className="h-4 w-4 text-primary" />
          {editable && editingTitle ? (
            <input
              value={titleDraft}
              onChange={(event) => setTitleDraft(event.target.value)}
              onBlur={saveTitle}
              onKeyDown={(event) => {
                if (event.key === 'Enter') saveTitle();
                if (event.key === 'Escape') {
                  setTitleDraft(title);
                  setEditingTitle(false);
                }
              }}
              className="h-7 min-w-36 flex-1 rounded border border-border bg-background px-2 text-sm font-medium outline-none focus:ring-2 focus:ring-primary/25"
              autoFocus
            />
          ) : (
            <button
              type="button"
              disabled={!editable}
              onClick={() => editable && setEditingTitle(true)}
              className="min-w-0 flex-1 truncate text-left text-sm font-medium text-foreground disabled:cursor-default"
            >
              {title}
            </button>
          )}
          <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-medium ${STATUS_STYLES[displayedStatus]}`}>
            {displayedStatus === 'approved' ? <Tick01Icon className="h-3 w-3" /> : displayedStatus === 'error' ? <AlertCircleIcon className="h-3 w-3" /> : null}
            {STATUS_LABELS[displayedStatus]}
          </span>
          {editable && focused && (
            <select
              value={status}
              onChange={(event) => updateAttributes({ status: event.target.value })}
              className="h-7 rounded border border-border bg-background px-2 text-xs outline-none focus:ring-2 focus:ring-primary/25"
            >
              <option value="draft">Draft</option>
              <option value="generated">Generated</option>
              <option value="needs_review">Needs review</option>
              <option value="approved">Approved</option>
              <option value="error">Error</option>
            </select>
          )}
          {editable && (
            <button
              type="button"
              onClick={regenerate}
              disabled={!selectedAgentId || !trimmedInstructions || busy !== null || Boolean(pendingRunId)}
              className="h-7 rounded border border-border bg-background px-2 text-xs font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
              contentEditable={false}
              title={!trimmedInstructions ? 'Add regeneration instructions first' : 'Regenerate this AI section'}
            >
              {busy === 'regenerate' ? 'Regenerating...' : 'Regenerate'}
            </button>
          )}
        </div>
        {editable && !candidate && (
          <div className="flex flex-wrap items-center gap-2 border-b border-border/60 bg-muted/15 px-3 py-2" contentEditable={false}>
            <select
              value={selectedAgentId}
              onChange={(event) => setSelectedAgentId(event.target.value)}
              disabled={Boolean(pendingRunId)}
              className="h-8 min-w-36 rounded border border-border bg-background px-2 text-xs outline-none focus:ring-2 focus:ring-primary/25"
            >
              {docAgents.length === 0 ? (
                <option value="">No document agent</option>
              ) : docAgents.map((agent) => (
                <option key={agent.id} value={agent.id}>{agent.name}</option>
              ))}
            </select>
            <input
              value={instructions}
              onChange={(event) => setInstructions(event.target.value)}
              placeholder="Required: what should this section say? Include URLs to research."
              disabled={Boolean(pendingRunId)}
              className="h-8 min-w-48 flex-1 rounded border border-border bg-background px-2 text-xs outline-none focus:ring-2 focus:ring-primary/25"
            />
            {pendingRunId && <span className="text-xs text-muted-foreground">Generating candidate...</span>}
          </div>
        )}
        {(ownerAgentName || model || displayedGeneratedAt || displayedSourceCount > 0) && (
          <div className="flex flex-wrap items-center gap-2 border-b border-border/60 bg-muted/20 px-3 py-1.5 text-[11px] text-muted-foreground" contentEditable={false}>
            {ownerAgentName && <span>{ownerAgentName}</span>}
            {model && <span>{model}</span>}
            {displayedGeneratedAt && <span>Generated {displayedGeneratedAt}</span>}
            {displayedSourceCount > 0 && (
              <span className="inline-flex items-center gap-1">
                <SourceCodeIcon className="h-3 w-3" />
                {displayedSourceCount} {displayedSourceCount === 1 ? 'source' : 'sources'}
              </span>
            )}
          </div>
        )}
        <div className="px-4 py-3 [&>*:last-child]:mb-0">
          <NodeViewContent />
        </div>
        {candidate && (
          <div className="border-t border-border bg-muted/20 px-4 py-3" contentEditable={false}>
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-foreground">Generated candidate</span>
              <span className="text-[11px] text-muted-foreground">
                {candidate.model || 'AI'}
                {candidate.prompt_hash ? ` / prompt ${candidate.prompt_hash.slice(0, 8)}` : ''}
              </span>
            </div>
            <div className="rounded border border-border bg-background">
              <div className="border-b border-border px-2 py-1.5 text-[11px] font-medium uppercase text-muted-foreground">Structured diff</div>
              <div className="max-h-64 overflow-auto">
                {diffChunks.map((chunk) => (
                  <div key={chunk.id} className="grid border-b border-border/60 last:border-b-0 md:grid-cols-2">
                    <div className={`min-h-10 whitespace-pre-wrap p-2 text-xs ${chunk.kind === 'added' ? 'bg-muted/30 text-muted-foreground' : chunk.kind === 'removed' || chunk.kind === 'changed' ? 'bg-red-50 text-red-900 dark:bg-red-950/20 dark:text-red-200' : 'text-muted-foreground'}`}>
                      <div className="mb-1 text-[10px] font-medium uppercase text-muted-foreground">Current</div>
                      {chunk.before || (chunk.kind === 'added' ? 'No prior text' : 'No visible text')}
                    </div>
                    <div className={`min-h-10 whitespace-pre-wrap p-2 text-xs ${chunk.kind === 'removed' ? 'bg-muted/30 text-muted-foreground' : chunk.kind === 'added' || chunk.kind === 'changed' ? 'bg-emerald-50 text-emerald-900 dark:bg-emerald-950/20 dark:text-emerald-200' : 'text-foreground'}`}>
                      <div className="mb-1 text-[10px] font-medium uppercase text-muted-foreground">Candidate</div>
                      {chunk.after || (chunk.kind === 'removed' ? 'Removed' : chunk.before || 'No visible text')}
                    </div>
                  </div>
                ))}
              </div>
            </div>
            {editable && (
              <div className="mt-3 flex justify-end gap-2">
                <button type="button" onClick={reject} disabled={busy !== null} className="h-8 rounded border border-border bg-background px-3 text-xs font-medium hover:bg-muted disabled:opacity-50">
                  {busy === 'reject' ? 'Rejecting...' : 'Reject'}
                </button>
                <button type="button" onClick={approve} disabled={busy !== null} className="h-8 rounded bg-primary px-3 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
                  {busy === 'approve' ? 'Approving...' : 'Approve'}
                </button>
              </div>
            )}
            {sources.length > 0 && (
              <div className="mt-3 rounded border border-border bg-background p-2">
                <div className="mb-1 text-[11px] font-medium uppercase text-muted-foreground">Sources</div>
                <div className="space-y-1">
                  {sources.slice(0, 5).map((source) => (
                    <div key={`${source.sourceType}:${source.sourceId}`} className="text-xs">
                      <span className="font-medium text-foreground">{source.title || source.sourceId}</span>
                      {sourceCanShowExcerpt(source) && <span className="text-muted-foreground"> - {source.excerpt}</span>}
                      {source.access === 'redacted' && <span className="text-muted-foreground"> - Restricted source</span>}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </section>
    </NodeViewWrapper>
  );
}
