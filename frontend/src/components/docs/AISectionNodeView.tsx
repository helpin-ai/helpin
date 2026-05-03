import { DOMSerializer, type Node as ProseMirrorNode } from '@tiptap/pm/model';
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { AiMagicIcon, Tick01Icon, AlertCircleIcon, SourceCodeIcon } from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
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
  beforeHtml?: string;
  afterHtml?: string;
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

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function nodeToHtml(node: ProseMirrorNode): string {
  try {
    const serializer = DOMSerializer.fromSchema(node.type.schema);
    const dom = serializer.serializeNode(node);
    const container = document.createElement('div');
    container.appendChild(dom);
    return container.innerHTML;
  } catch {
    return escapeHtml(node.textContent);
  }
}

function diffBlocks(beforeNode: ProseMirrorNode, afterNode: ProseMirrorNode): AISectionDiffChunk[] {
  const beforeBlocks: ProseMirrorNode[] = [];
  const afterBlocks: ProseMirrorNode[] = [];
  beforeNode.content.forEach((child) => beforeBlocks.push(child));
  afterNode.content.forEach((child) => afterBlocks.push(child));

  const beforeKeys = beforeBlocks.map((node) => JSON.stringify(node.toJSON()));
  const afterKeys = afterBlocks.map((node) => JSON.stringify(node.toJSON()));

  const m = beforeKeys.length;
  const n = afterKeys.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      dp[i][j] = beforeKeys[i - 1] === afterKeys[j - 1]
        ? dp[i - 1][j - 1] + 1
        : Math.max(dp[i - 1][j], dp[i][j - 1]);
    }
  }

  const ops: { kind: 'unchanged' | 'added' | 'removed'; node: ProseMirrorNode }[] = [];
  let i = m;
  let j = n;
  while (i > 0 && j > 0) {
    if (beforeKeys[i - 1] === afterKeys[j - 1]) {
      ops.unshift({ kind: 'unchanged', node: beforeBlocks[i - 1] });
      i--; j--;
    } else if (dp[i - 1][j] >= dp[i][j - 1]) {
      ops.unshift({ kind: 'removed', node: beforeBlocks[i - 1] });
      i--;
    } else {
      ops.unshift({ kind: 'added', node: afterBlocks[j - 1] });
      j--;
    }
  }
  while (i > 0) { i--; ops.unshift({ kind: 'removed', node: beforeBlocks[i] }); }
  while (j > 0) { j--; ops.unshift({ kind: 'added', node: afterBlocks[j] }); }

  return ops.map((op, index) => {
    const html = nodeToHtml(op.node);
    if (op.kind === 'unchanged') return { id: `b-${index}`, kind: 'unchanged', beforeHtml: html };
    if (op.kind === 'added') return { id: `b-${index}`, kind: 'added', afterHtml: html };
    return { id: `b-${index}`, kind: 'removed', beforeHtml: html };
  });
}

function fallbackAISectionDiff(currentContent: unknown, candidate: DocsAISectionCandidate | null): AISectionDiffChunk[] {
  const before = extractPlainText(currentContent || candidate?.current_content).trim();
  const after = (candidate?.candidate_text || extractPlainText(candidate?.candidate_content)).trim();
  if (!before && !after) {
    return [{ id: 'empty', kind: 'unchanged', beforeHtml: 'No visible text changes.' }];
  }
  if (before === after) {
    return [{ id: 'same', kind: 'unchanged', beforeHtml: escapeHtml(before) || 'No visible text changes.' }];
  }
  return [{
    id: 'fallback',
    kind: before && after ? 'changed' : before ? 'removed' : 'added',
    beforeHtml: escapeHtml(before),
    afterHtml: escapeHtml(after),
  }];
}

function buildAISectionDiff(editor: NodeViewProps['editor'], currentContent: unknown, candidate: DocsAISectionCandidate | null): AISectionDiffChunk[] {
  if (!candidate?.candidate_content) return [];
  try {
    const before = editor.schema.nodeFromJSON((candidate.current_content || currentContent) as Record<string, unknown>);
    const after = editor.schema.nodeFromJSON(candidate.candidate_content as Record<string, unknown>);
    const chunks = diffBlocks(before, after);
    if (chunks.length === 0 || chunks.every((chunk) => chunk.kind === 'unchanged')) {
      return chunks.length > 0 ? chunks : [{ id: 'same', kind: 'unchanged', beforeHtml: 'No visible text changes.' }];
    }
    return chunks;
  } catch {
    return fallbackAISectionDiff(currentContent, candidate);
  }
}

const BLOCK_BASE = 'prose prose-sm dark:prose-invert max-w-none [&>*:first-child]:mt-0 [&>*:last-child]:mb-0';
const ADDED_BLOCK = `${BLOCK_BASE} rounded-sm bg-emerald-50/70 px-2 py-1 text-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-100`;
const REMOVED_BLOCK = `${BLOCK_BASE} rounded-sm bg-red-50/70 px-2 py-1 text-red-900 line-through decoration-red-600/70 decoration-1 dark:bg-red-950/30 dark:text-red-100 dark:decoration-red-400/70`;
const UNCHANGED_BLOCK = `${BLOCK_BASE} text-foreground`;

function BlockDiffEntry({ chunk }: { chunk: AISectionDiffChunk }) {
  if (chunk.kind === 'added') {
    return <ins className={`block ${ADDED_BLOCK} no-underline`} dangerouslySetInnerHTML={{ __html: chunk.afterHtml || '' }} />;
  }
  if (chunk.kind === 'removed') {
    return <del className={`block ${REMOVED_BLOCK}`} dangerouslySetInnerHTML={{ __html: chunk.beforeHtml || '' }} />;
  }
  if (chunk.kind === 'changed') {
    return (
      <div className="space-y-1">
        {chunk.beforeHtml ? <del className={`block ${REMOVED_BLOCK}`} dangerouslySetInnerHTML={{ __html: chunk.beforeHtml }} /> : null}
        {chunk.afterHtml ? <ins className={`block ${ADDED_BLOCK} no-underline`} dangerouslySetInnerHTML={{ __html: chunk.afterHtml }} /> : null}
      </div>
    );
  }
  return <div className={UNCHANGED_BLOCK} dangerouslySetInnerHTML={{ __html: chunk.beforeHtml || '' }} />;
}

function InlineAISectionDiff({ chunks }: { chunks: AISectionDiffChunk[] }) {
  return (
    <div className="space-y-1.5 text-sm leading-6">
      {chunks.map((chunk) => (
        <BlockDiffEntry key={chunk.id} chunk={chunk} />
      ))}
    </div>
  );
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
  const selectedDocAgentId = docAgents.some((agent) => agent.id === selectedAgentId)
    ? selectedAgentId
    : docAgents[0]?.id ?? '';
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
    if (!workspaceId || !documentId || !blockId || !selectedDocAgentId) return;
    if (!trimmedInstructions) {
      setFocused(true);
      toast.error('Add regeneration instructions first');
      return;
    }
    setBusy('regenerate');
    const { data, error } = await docsService.regenerateAISection(workspaceId, documentId, blockId, {
      agent_id: selectedDocAgentId,
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
        className={`docs-ai-section group/ai-section relative my-5 rounded-sm border-l-2 pl-4 transition-colors ${
          focused ? 'border-primary/60' : 'border-primary/20 hover:border-primary/40'
        }`}
        data-ai-section=""
        data-ai-section-status={displayedStatus}
      >
        <div
          className={`flex flex-wrap items-center gap-2 px-1 py-1.5 transition-opacity ${
            focused || candidate ? 'opacity-100' : 'opacity-0 group-hover/ai-section:opacity-100'
          }`}
          contentEditable={false}
        >
          <AiMagicIcon className="h-4 w-4 text-primary" />
          {editable && editingTitle ? (
            <Input
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
              className="h-7 min-w-36 flex-1 text-sm font-medium"
              autoFocus
            />
          ) : (
            <button
              type="button"
              disabled={!editable}
              onClick={() => {
                if (!editable) return;
                setTitleDraft(title);
                setEditingTitle(true);
              }}
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
            <Select value={status} onValueChange={(value) => updateAttributes({ status: value })}>
              <SelectTrigger size="sm" className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="draft">Draft</SelectItem>
                <SelectItem value="generated">Generated</SelectItem>
                <SelectItem value="needs_review">Needs review</SelectItem>
                <SelectItem value="approved">Approved</SelectItem>
                <SelectItem value="error">Error</SelectItem>
              </SelectContent>
            </Select>
          )}
          {editable && candidate && (
            <>
              <Button type="button" size="sm" variant="outline" onClick={reject} disabled={busy !== null}>
                {busy === 'reject' ? 'Rejecting...' : 'Reject'}
              </Button>
              <Button type="button" size="sm" onClick={approve} disabled={busy !== null}>
                {busy === 'approve' ? 'Approving...' : 'Approve'}
              </Button>
            </>
          )}
          {editable && !candidate && (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={regenerate}
              disabled={!selectedDocAgentId || !trimmedInstructions || busy !== null || Boolean(pendingRunId)}
              contentEditable={false}
              title={!trimmedInstructions ? 'Add regeneration instructions first' : 'Regenerate this AI section'}
            >
              {busy === 'regenerate' ? 'Regenerating...' : 'Regenerate'}
            </Button>
          )}
        </div>
        {editable && !candidate && (
          <div
            className={`flex flex-wrap items-center gap-2 px-1 py-1.5 transition-opacity ${
              focused ? 'opacity-100' : 'opacity-0 group-hover/ai-section:opacity-100'
            }`}
            contentEditable={false}
          >
            <Select
              value={selectedDocAgentId}
              onValueChange={(value) => setSelectedAgentId(value)}
              disabled={Boolean(pendingRunId) || docAgents.length === 0}
            >
              <SelectTrigger size="sm" className="min-w-36">
                <SelectValue placeholder={docAgents.length === 0 ? 'No document agent' : 'Select agent'} />
              </SelectTrigger>
              <SelectContent>
                {docAgents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              value={instructions}
              onChange={(event) => setInstructions(event.target.value)}
              placeholder="Required: what should this section say? Include URLs to research."
              disabled={Boolean(pendingRunId)}
              className="h-8 min-w-48 flex-1 text-xs"
            />
            {pendingRunId && (
              <span className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                Generating
                <span className="inline-flex items-center gap-0.5 text-base leading-none">
                  <span className="animate-bounce [animation-delay:0ms] [animation-duration:1s]">.</span>
                  <span className="animate-bounce [animation-delay:150ms] [animation-duration:1s]">.</span>
                  <span className="animate-bounce [animation-delay:300ms] [animation-duration:1s]">.</span>
                </span>
              </span>
            )}
          </div>
        )}
        {(ownerAgentName || model || displayedGeneratedAt || displayedSourceCount > 0) && (
          <div
            className={`flex flex-wrap items-center gap-2 px-1 py-1 text-[11px] text-muted-foreground transition-opacity ${
              focused ? 'opacity-100' : 'opacity-0 group-hover/ai-section:opacity-100'
            }`}
            contentEditable={false}
          >
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
        <div className={candidate ? 'hidden' : 'py-1 [&>*:last-child]:mb-0'}>
          <NodeViewContent />
        </div>
        {candidate && (
          <div className="py-1" contentEditable={false}>
            <InlineAISectionDiff chunks={diffChunks} />
            {sources.length > 0 && (
              <div className="mt-3 space-y-1 border-t border-border/60 pt-2">
                <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Sources</div>
                {sources.slice(0, 5).map((source) => (
                  <div key={`${source.sourceType}:${source.sourceId}`} className="text-xs">
                    <span className="font-medium text-foreground">{source.title || source.sourceId}</span>
                    {sourceCanShowExcerpt(source) && <span className="text-muted-foreground"> - {source.excerpt}</span>}
                    {source.access === 'redacted' && <span className="text-muted-foreground"> - Restricted source</span>}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </section>
    </NodeViewWrapper>
  );
}
