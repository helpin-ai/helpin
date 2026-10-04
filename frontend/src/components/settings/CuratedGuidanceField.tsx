import { useState } from 'react';
import { Delete01Icon, PencilEdit01Icon, PinIcon, PlusSignIcon } from '@/lib/icons';
import type {
  CreateCuratedGuidanceRequest,
  CuratedGuidance,
  CuratedGuidanceStatus,
  UpdateCuratedGuidanceRequest,
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type GuidanceDraft = {
  title: string;
  questionPatterns: string;
  answer: string;
  status: CuratedGuidanceStatus;
};

const emptyDraft: GuidanceDraft = {
  title: '',
  questionPatterns: '',
  answer: '',
  status: 'active',
};

function guidanceDraft(item?: CuratedGuidance): GuidanceDraft {
  if (!item) return emptyDraft;
  return {
    title: item.title,
    questionPatterns: item.question_patterns.join('\n'),
    answer: item.answer,
    status: item.status,
  };
}

function editablePayloadFromDraft(draft: GuidanceDraft) {
  return {
    title: draft.title.trim(),
    question_patterns: draft.questionPatterns.split('\n').map((value) => value.trim()).filter(Boolean),
    answer: draft.answer.trim(),
  };
}

function createPayloadFromDraft(draft: GuidanceDraft): CreateCuratedGuidanceRequest {
  return {
    ...editablePayloadFromDraft(draft),
    language: '',
    intent: 'unknown',
    topics: [],
  };
}

function updatePayloadFromDraft(draft: GuidanceDraft): UpdateCuratedGuidanceRequest {
  return {
    ...editablePayloadFromDraft(draft),
    status: draft.status,
  };
}

function draftsMatch(left: GuidanceDraft, right: GuidanceDraft) {
  return left.title === right.title
    && left.questionPatterns === right.questionPatterns
    && left.answer === right.answer
    && left.status === right.status;
}

function GuidanceEditor({
  draft,
  editingExisting,
  saving,
  valid,
  onChange,
  onCancel,
  onSave,
}: {
  draft: GuidanceDraft;
  editingExisting: boolean;
  saving: boolean;
  valid: boolean;
  onChange: (change: Partial<GuidanceDraft>) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const idPrefix = editingExisting ? 'edit-guidance' : 'new-guidance';

  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        onSave();
      }}
    >
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-title`}>Title</Label>
        <Input
          id={`${idPrefix}-title`}
          value={draft.title}
          onChange={(event) => onChange({ title: event.target.value })}
          placeholder="Current pricing"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-answer`}>Preferred answer</Label>
        <Textarea
          id={`${idPrefix}-answer`}
          value={draft.answer}
          onChange={(event) => onChange({ answer: event.target.value })}
          placeholder="Describe when to use this answer and how Helpin should respond. Include key facts, conditions or exceptions, and when to hand off to your team."
          rows={8}
          className="min-h-40 rounded-lg border-border bg-background"
        />
      </div>

      <details className="text-sm">
        <summary className="w-fit cursor-pointer rounded-sm text-xs font-medium text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Additional options</summary>
        <div className="mt-3 space-y-2">
          <Label htmlFor={`${idPrefix}-patterns`}>Example questions (optional)</Label>
          <Textarea
            id={`${idPrefix}-patterns`}
            value={draft.questionPatterns}
            onChange={(event) => onChange({ questionPatterns: event.target.value })}
            placeholder={'What is your pricing?\nHow much does Usermaven cost?'}
            rows={3}
            className="max-h-64 overflow-y-auto rounded-lg border-border bg-background"
          />
          <p className="text-xs text-muted-foreground">Add examples to help match unusual wording. One question per line.</p>
        </div>
      </details>

      {editingExisting ? (
        <div className="max-w-xs space-y-2">
          <Label htmlFor={`${idPrefix}-status`}>Status</Label>
          <Select value={draft.status} onValueChange={(value: CuratedGuidanceStatus) => onChange({ status: value })}>
            <SelectTrigger id={`${idPrefix}-status`}><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="active">Active</SelectItem><SelectItem value="disabled">Disabled</SelectItem></SelectContent>
          </Select>
        </div>
      ) : null}

      <div className="flex justify-end gap-2 border-t pt-4">
        <Button type="button" variant="outline" disabled={saving} onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={!valid || saving}>{saving ? 'Saving…' : editingExisting ? 'Save changes' : 'Save answer'}</Button>
      </div>
    </form>
  );
}

export function CuratedGuidanceField({ workspaceId, agentId }: { workspaceId: string; agentId?: string }) {
  const { data: items = [], isLoading } = useCuratedGuidance(workspaceId, agentId);
  const createGuidance = useCreateCuratedGuidance(workspaceId);
  const updateGuidance = useUpdateCuratedGuidance(workspaceId);
  const deleteGuidance = useDeleteCuratedGuidance(workspaceId);
  const [editing, setEditing] = useState<CuratedGuidance | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<CuratedGuidance | null>(null);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [draft, setDraft] = useState<GuidanceDraft>(emptyDraft);

  const openEditor = (item: CuratedGuidance | null) => {
    setEditing(item);
    setDraft(guidanceDraft(item ?? undefined));
  };

  const closeEditor = () => {
    setConfirmDiscard(false);
    setEditing(undefined);
    setDraft(emptyDraft);
  };

  const requestCloseEditor = () => {
    if (createGuidance.isPending || updateGuidance.isPending) return;
    if (editing !== undefined && !draftsMatch(draft, guidanceDraft(editing ?? undefined))) {
      setConfirmDiscard(true);
      return;
    }
    closeEditor();
  };

  const save = async () => {
    if (!agentId) return;
    try {
      if (editing) {
        await updateGuidance.mutateAsync({
          agentId,
          guidanceId: editing.id,
          payload: updatePayloadFromDraft(draft),
        });
      } else {
        await createGuidance.mutateAsync({ agentId, payload: createPayloadFromDraft(draft) });
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
  const valid = draft.title.trim().length > 0 && draft.answer.trim().length > 0;
  const editorOpen = editing !== undefined;
  const updateDraft = (change: Partial<GuidanceDraft>) => setDraft((current) => ({ ...current, ...change }));

  return (
    <>
      <div className="overflow-hidden rounded-lg border border-border/70 bg-card">
        <div className="flex items-start justify-between gap-4 p-4">
          <div>
            <p className="text-sm font-medium">Preferred answers</p>
            <p className="mt-0.5 text-sm text-muted-foreground">
              Tell Helpin when and how to respond to recurring or sensitive questions.
            </p>
          </div>
          <Button type="button" size="sm" variant="outline" disabled={!agentId || editorOpen} onClick={() => openEditor(null)}>
            <PlusSignIcon className="h-4 w-4" />
            Add preferred answer
          </Button>
        </div>

        <div className="border-t border-border px-4 py-4">
          {!agentId ? (
            <div className="rounded-lg border border-dashed px-4 py-6 text-center">
              <p className="text-sm font-medium">Select a support agent first</p>
              <p className="mx-auto mt-1 max-w-lg text-xs leading-5 text-muted-foreground">
                Choose an agent in the Setup tab, then add its preferred answers.
              </p>
            </div>
          ) : isLoading ? (
            <div className="space-y-2"><Skeleton className="h-20 rounded-lg" /><Skeleton className="h-20 rounded-lg" /></div>
          ) : items.length === 0 ? (
            <div className="rounded-lg border border-dashed px-4 py-6 text-center">
              <p className="text-sm font-medium">No preferred answers yet</p>
              <p className="mx-auto mt-1 max-w-lg text-xs leading-5 text-muted-foreground">
                Add answers for pricing, refunds, plan limits, security, and company policies.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {items.length > 0 ? (
                <div className="divide-y rounded-lg border">
                  {items.map((item) => (
                    <div key={item.id} className="flex items-start gap-3 p-3">
                      <PinIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-medium">{item.title}</p>
                          {item.status === 'disabled' ? <Badge variant="outline">Disabled</Badge> : null}
                        </div>
                        <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{item.answer}</p>
                        {item.question_patterns.length > 0 && (
                          <details className="mt-1 text-xs text-muted-foreground">
                            <summary className="w-fit cursor-pointer">Example questions</summary>
                            <ul className="mt-1 space-y-1">{item.question_patterns.map((question, index) => <li key={index}>{question}</li>)}</ul>
                          </details>
                        )}
                      </div>
                      <div className="flex shrink-0 gap-1">
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" disabled={editorOpen} aria-label={`Edit ${item.title}`} onClick={() => openEditor(item)}>
                          <PencilEdit01Icon className="h-3.5 w-3.5" />
                        </Button>
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-destructive" disabled={editorOpen} aria-label={`Delete ${item.title}`} onClick={() => setDeleting(item)}>
                          <Delete01Icon className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>

      <Dialog open={editorOpen} onOpenChange={(open) => { if (!open) requestCloseEditor(); }}>
        <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader className="pr-8">
            <DialogTitle>{editing ? 'Edit preferred answer' : 'New preferred answer'}</DialogTitle>
            <DialogDescription>
              Helpin adapts your instructions to the customer’s question and language.
            </DialogDescription>
          </DialogHeader>
          <GuidanceEditor
            draft={draft}
            editingExisting={Boolean(editing)}
            saving={saving}
            valid={valid}
            onChange={updateDraft}
            onCancel={requestCloseEditor}
            onSave={() => void save()}
          />
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmDiscard} onOpenChange={setConfirmDiscard}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard unsaved changes?</AlertDialogTitle>
            <AlertDialogDescription>Your changes to this preferred answer have not been saved.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep editing</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={closeEditor}>Discard changes</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={Boolean(deleting)} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Remove preferred answer?</AlertDialogTitle><AlertDialogDescription>Helpin will immediately stop using “{deleting?.title}” when answering customer questions.</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter><AlertDialogCancel disabled={deleteGuidance.isPending}>Cancel</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={deleteGuidance.isPending} onClick={() => void remove()}>{deleteGuidance.isPending ? 'Removing…' : 'Remove'}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
