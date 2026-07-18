import { useState } from 'react';
import { Delete01Icon, PencilEdit01Icon, PinIcon, PlusSignIcon } from '@/lib/icons';
import type {
  CreateCuratedGuidanceRequest,
  CuratedGuidance,
  CuratedGuidanceIntent,
  CuratedGuidanceStatus,
} from '@/lib/pmTypes';
import {
  useCreateCuratedGuidance,
  useCuratedGuidance,
  useDeleteCuratedGuidance,
  useUpdateCuratedGuidance,
} from '@/hooks/queries/useSupport';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';

const intentLabels: Record<CuratedGuidanceIntent, string> = {
  pricing_general: 'Pricing',
  plan_recommendation: 'Plan recommendation',
  billing_tax: 'Billing & tax',
  unknown: 'General',
};

type GuidanceDraft = {
  title: string;
  questionPatterns: string;
  answer: string;
  intent: CuratedGuidanceIntent;
  language: string;
  status: CuratedGuidanceStatus;
};

const emptyDraft: GuidanceDraft = {
  title: '',
  questionPatterns: '',
  answer: '',
  intent: 'unknown',
  language: '',
  status: 'active',
};

function guidanceDraft(item?: CuratedGuidance): GuidanceDraft {
  if (!item) return emptyDraft;
  return {
    title: item.title,
    questionPatterns: item.question_patterns.join('\n'),
    answer: item.answer,
    intent: item.intent,
    language: item.language,
    status: item.status,
  };
}

function payloadFromDraft(draft: GuidanceDraft): CreateCuratedGuidanceRequest {
  return {
    title: draft.title.trim(),
    question_patterns: draft.questionPatterns.split('\n').map((value) => value.trim()).filter(Boolean),
    answer: draft.answer.trim(),
    intent: draft.intent,
    topics: [],
    language: draft.language.trim().toLowerCase(),
  };
}

export function CuratedGuidanceField({ workspaceId, agentId }: { workspaceId: string; agentId?: string }) {
  const { data: items = [], isLoading } = useCuratedGuidance(workspaceId, agentId);
  const createGuidance = useCreateCuratedGuidance(workspaceId);
  const updateGuidance = useUpdateCuratedGuidance(workspaceId);
  const deleteGuidance = useDeleteCuratedGuidance(workspaceId);
  const [editing, setEditing] = useState<CuratedGuidance | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<CuratedGuidance | null>(null);
  const [draft, setDraft] = useState<GuidanceDraft>(emptyDraft);

  const openEditor = (item: CuratedGuidance | null) => {
    setEditing(item);
    setDraft(guidanceDraft(item ?? undefined));
  };

  const closeEditor = () => {
    setEditing(undefined);
    setDraft(emptyDraft);
  };

  const save = async () => {
    if (!agentId) return;
    const payload = payloadFromDraft(draft);
    try {
      if (editing) {
        await updateGuidance.mutateAsync({
          agentId,
          guidanceId: editing.id,
          payload: { ...payload, status: draft.status },
        });
      } else {
        await createGuidance.mutateAsync({ agentId, payload });
      }
      closeEditor();
    } catch {
      // The mutation owns the error toast; keep the editor open for correction.
    }
  };

  const remove = async () => {
    if (!agentId || !deleting) return;
    try {
      await deleteGuidance.mutateAsync({ agentId, guidanceId: deleting.id });
      setDeleting(null);
    } catch {
      // The mutation owns the error toast; leave confirmation open for retry.
    }
  };

  const saving = createGuidance.isPending || updateGuidance.isPending;
  const valid = draft.title.trim().length > 0 && draft.answer.trim().length > 0 && draft.questionPatterns.trim().length > 0;

  return (
    <>
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-sm font-medium">Pinned answers</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Short canonical answers outrank longer docs for matching questions and are never shown as citations.
            </p>
          </div>
          <Button type="button" size="sm" variant="outline" disabled={!agentId} onClick={() => openEditor(null)}>
            <PlusSignIcon className="h-4 w-4" />
            Pin answer
          </Button>
        </div>

        {isLoading ? (
          <div className="space-y-2"><Skeleton className="h-20 rounded-lg" /><Skeleton className="h-20 rounded-lg" /></div>
        ) : items.length === 0 ? (
          <div className="rounded-lg border border-dashed px-4 py-6 text-center">
            <PinIcon className="mx-auto h-4 w-4 text-muted-foreground" />
            <p className="mt-2 text-sm font-medium">No pinned answers</p>
            <p className="mt-1 text-xs text-muted-foreground">Pin answers for pricing, policies, and other high-value questions.</p>
          </div>
        ) : (
          <div className="divide-y rounded-lg border">
            {items.map((item) => (
              <div key={item.id} className="flex items-start gap-3 p-3">
                <PinIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{item.title}</p>
                    <Badge variant="secondary">{intentLabels[item.intent]}</Badge>
                    {item.status === 'disabled' ? <Badge variant="outline">Disabled</Badge> : null}
                  </div>
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{item.answer}</p>
                  <p className="mt-1 truncate text-xs text-muted-foreground">Matches: {item.question_patterns.join(' · ')}</p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8" aria-label={`Edit ${item.title}`} onClick={() => openEditor(item)}>
                    <PencilEdit01Icon className="h-3.5 w-3.5" />
                  </Button>
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-destructive" aria-label={`Delete ${item.title}`} onClick={() => setDeleting(item)}>
                    <Delete01Icon className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={editing !== undefined} onOpenChange={(open) => !open && closeEditor()}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{editing ? 'Edit pinned answer' : 'Pin an answer'}</DialogTitle>
            <DialogDescription>Use exact, customer-safe wording. Include concrete prices, limits, and qualifiers when relevant.</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-1">
            <div className="space-y-2">
              <Label htmlFor="guidance-title">Title</Label>
              <Input id="guidance-title" value={draft.title} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} placeholder="Current pricing" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="guidance-patterns">Example questions</Label>
              <Textarea id="guidance-patterns" value={draft.questionPatterns} onChange={(event) => setDraft((current) => ({ ...current, questionPatterns: event.target.value }))} placeholder={'What is your pricing?\nHow much does Usermaven cost?'} rows={3} />
              <p className="text-xs text-muted-foreground">One question per line.</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="guidance-answer">Canonical answer</Label>
              <Textarea id="guidance-answer" value={draft.answer} onChange={(event) => setDraft((current) => ({ ...current, answer: event.target.value }))} placeholder="State the answer exactly as customers should receive it." rows={7} />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>Intent</Label>
                <Select value={draft.intent} onValueChange={(value: CuratedGuidanceIntent) => setDraft((current) => ({ ...current, intent: value }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>{Object.entries(intentLabels).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="guidance-language">Language</Label>
                <Input id="guidance-language" value={draft.language} onChange={(event) => setDraft((current) => ({ ...current, language: event.target.value }))} placeholder="en (optional)" />
              </div>
            </div>
            {editing ? (
              <div className="space-y-2">
                <Label>Status</Label>
                <Select value={draft.status} onValueChange={(value: CuratedGuidanceStatus) => setDraft((current) => ({ ...current, status: value }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent><SelectItem value="active">Active</SelectItem><SelectItem value="disabled">Disabled</SelectItem></SelectContent>
                </Select>
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeEditor}>Cancel</Button>
            <Button type="button" disabled={!valid || saving} onClick={() => void save()}>{saving ? 'Saving…' : editing ? 'Save changes' : 'Pin answer'}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(deleting)} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Remove pinned answer?</AlertDialogTitle><AlertDialogDescription>The AI will immediately stop using “{deleting?.title}” as canonical guidance.</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter><AlertDialogCancel disabled={deleteGuidance.isPending}>Cancel</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={deleteGuidance.isPending} onClick={() => void remove()}>{deleteGuidance.isPending ? 'Removing…' : 'Remove'}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
