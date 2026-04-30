import { useMemo, useRef, useState } from 'react';
import type { Editor } from '@tiptap/react';
import { toast } from 'sonner';
import { SettingsPageFrame } from './SettingsPageFrame';
import { AddShortcutDialog } from '@/components/support/AddShortcutDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import {
  useCannedResponses,
  useDeleteCannedResponse,
  useUpdateCannedResponse,
} from '@/hooks/queries/useSupport';
import type { SupportCannedResponse } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  Cancel01Icon,
  Delete01Icon,
  Loading01Icon,
  MoreHorizontalIcon,
  PencilEdit01Icon,
  PlusSignIcon,
  Search01Icon,
  StickyNote01Icon,
  TickDouble01Icon,
} from '@/lib/icons';

const DEFAULT_TAG = 'Others';
const BUILT_IN_TAGS = [DEFAULT_TAG, 'Support', 'Sales', 'Billing'];
const CUSTOM_TAG_VALUE = '__custom_tag__';

type ShortcutFormState = {
  shortCode: string;
  title: string;
  tag: string;
  customTag: string;
  content: string;
};

const emptyForm: ShortcutFormState = {
  shortCode: '',
  title: '',
  tag: DEFAULT_TAG,
  customTag: '',
  content: '',
};

function normalizeTag(state: Pick<ShortcutFormState, 'tag' | 'customTag'>) {
  if (state.tag === CUSTOM_TAG_VALUE) return state.customTag.trim() || DEFAULT_TAG;
  return state.tag.trim() || DEFAULT_TAG;
}

function stripHTML(value: string) {
  if (!value) return '';
  const doc = new DOMParser().parseFromString(value, 'text/html');
  return doc.body.textContent?.replace(/\s+/g, ' ').trim() ?? '';
}

function validateShortcut(shortCode: string) {
  const value = shortCode.trim();
  if (!value) return 'Shortcut is required';
  if (!value.startsWith('!')) return 'Shortcut must start with !';
  if (/\s/.test(value)) return 'Shortcut cannot contain spaces';
  if (value.length < 2) return 'Shortcut must have at least one character after !';
  return null;
}

function buildTagOptions(responses: SupportCannedResponse[]) {
  return Array.from(new Set([...BUILT_IN_TAGS, ...responses.map((item) => item.tag || DEFAULT_TAG)]));
}

function groupResponses(responses: SupportCannedResponse[]) {
  const groups = new Map<string, SupportCannedResponse[]>();
  for (const response of responses) {
    const tag = response.tag || DEFAULT_TAG;
    groups.set(tag, [...(groups.get(tag) ?? []), response]);
  }
  return Array.from(groups.entries())
    .map(([tag, items]) => ({
      tag,
      items: items.sort((a, b) => a.short_code.localeCompare(b.short_code)),
    }))
    .sort((a, b) => (a.tag === DEFAULT_TAG ? 1 : b.tag === DEFAULT_TAG ? -1 : a.tag.localeCompare(b.tag)));
}

function ShortcutTagSelect({
  value,
  customValue,
  tags,
  onTagChange,
  onCustomChange,
}: {
  value: string;
  customValue: string;
  tags: string[];
  onTagChange: (value: string) => void;
  onCustomChange: (value: string) => void;
}) {
  return (
    <div className="space-y-2">
      <Select value={value} onValueChange={onTagChange}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {tags.map((tag) => (
            <SelectItem key={tag} value={tag}>
              {tag === DEFAULT_TAG ? 'Others (default)' : tag}
            </SelectItem>
          ))}
          <SelectItem value={CUSTOM_TAG_VALUE}>Create new tag</SelectItem>
        </SelectContent>
      </Select>
      {value === CUSTOM_TAG_VALUE ? (
        <Input
          value={customValue}
          onChange={(event) => onCustomChange(event.target.value)}
          placeholder="Tag name"
        />
      ) : null}
    </div>
  );
}

function ShortcutForm({
  state,
  workspaceId,
  tags,
  existing,
  editingId,
  pending,
  submitLabel,
  onChange,
  onSubmit,
  onCancel,
}: {
  state: ShortcutFormState;
  workspaceId: string;
  tags: string[];
  existing: SupportCannedResponse[];
  editingId?: string;
  pending: boolean;
  submitLabel: string;
  onChange: (state: ShortcutFormState) => void;
  onSubmit: (markdownContent: string) => void;
  onCancel?: () => void;
}) {
  const [pendingUploads, setPendingUploads] = useState(0);
  const editorRef = useRef<Editor | null>(null);

  // The TiptapEditor emits HTML through onChange, but the composer inserts
  // shortcut content via the markdown extension. Read markdown from the
  // editor at submit time so saved content stays in the same shape as
  // composer-inserted shortcuts (matches AddShortcutDialog's create path).
  const submitWithMarkdown = () => {
    const editor = editorRef.current;
    const storage = editor
      ? (editor.storage as { markdown?: { getMarkdown(): string } }).markdown
      : undefined;
    const markdown = storage?.getMarkdown ? storage.getMarkdown().trim() : stripHTML(state.content);
    onSubmit(markdown);
  };
  const shortcutError = validateShortcut(state.shortCode);
  const normalizedCode = state.shortCode.trim();
  const duplicate = existing.some((item) => item.id !== editingId && item.short_code === normalizedCode);
  const contentText = stripHTML(state.content);
  const customTagMissing = state.tag === CUSTOM_TAG_VALUE && !state.customTag.trim();
  const canSubmit = !shortcutError && !duplicate && !customTagMissing && contentText.length > 0 && !pending && pendingUploads === 0;

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-[minmax(160px,0.8fr)_minmax(180px,1fr)_minmax(180px,0.9fr)]">
        <div className="space-y-2">
          <Label>Shortcut</Label>
          <Input
            value={state.shortCode}
            onChange={(event) => onChange({ ...state, shortCode: event.target.value })}
            placeholder="!bang"
            className="font-mono"
          />
          {shortcutError ? <p className="text-xs text-destructive">{shortcutError}</p> : null}
          {duplicate ? <p className="text-xs text-destructive">That shortcut already exists.</p> : null}
        </div>
        <div className="space-y-2">
          <Label>Name</Label>
          <Input
            value={state.title}
            onChange={(event) => onChange({ ...state, title: event.target.value })}
            placeholder="Greeting"
          />
        </div>
        <div className="space-y-2">
          <Label>In tag</Label>
          <ShortcutTagSelect
            value={state.tag}
            customValue={state.customTag}
            tags={tags}
            onTagChange={(tag) => onChange({ ...state, tag })}
            onCustomChange={(customTag) => onChange({ ...state, customTag })}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Message</Label>
        <TiptapEditor
          content={state.content}
          onChange={(content) => onChange({ ...state, content })}
          placeholder="Write the saved reply..."
          className="rounded-lg border-border bg-background"
          uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
          onUploadStateChange={setPendingUploads}
          onEditorReady={(editor) => { editorRef.current = editor; }}
        />
        {pendingUploads > 0 ? <p className="text-xs text-muted-foreground">Uploading attachments...</p> : null}
      </div>
      <div className="flex items-center justify-end gap-2">
        {onCancel ? (
          <Button type="button" variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
        <Button type="button" disabled={!canSubmit} onClick={submitWithMarkdown}>
          {pending ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlusSignIcon className="h-4 w-4" />}
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}

function formFromResponse(response: SupportCannedResponse): ShortcutFormState {
  return {
    shortCode: response.short_code,
    title: response.title,
    tag: response.tag || DEFAULT_TAG,
    customTag: '',
    content: response.content,
  };
}

export function MessageShortcutsSettingsPage() {
  return (
    <SettingsPageFrame section="message-shortcuts">
      {({ workspaceId }) => <MessageShortcutsSettingsContent workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}

function MessageShortcutsSettingsContent({ workspaceId }: { workspaceId: string }) {
  const { data: responses = [], isLoading } = useCannedResponses(workspaceId);
  const updateShortcut = useUpdateCannedResponse(workspaceId);
  const deleteShortcut = useDeleteCannedResponse(workspaceId);
  const [addOpen, setAddOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingForm, setEditingForm] = useState<ShortcutFormState>(emptyForm);
  const [pendingDelete, setPendingDelete] = useState<SupportCannedResponse | null>(null);
  const [pendingGroupDelete, setPendingGroupDelete] = useState<string | null>(null);
  const [renamingTag, setRenamingTag] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');

  const tags = useMemo(() => buildTagOptions(responses), [responses]);
  const filteredResponses = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return responses;
    return responses.filter((item) =>
      item.short_code.toLowerCase().includes(query) ||
      item.title.toLowerCase().includes(query) ||
      stripHTML(item.content).toLowerCase().includes(query) ||
      (item.tag || DEFAULT_TAG).toLowerCase().includes(query),
    );
  }, [responses, search]);
  const groups = useMemo(() => groupResponses(filteredResponses), [filteredResponses]);

  const handleUpdate = async (responseId: string, markdownContent: string) => {
    const tag = normalizeTag(editingForm);
    try {
      await updateShortcut.mutateAsync({
        responseId,
        payload: {
          short_code: editingForm.shortCode.trim(),
          title: editingForm.title.trim() || editingForm.shortCode.trim(),
          content: markdownContent,
          tag,
        },
      });
      setEditingId(null);
      toast.success('Shortcut updated');
    } catch {}
  };

  const startRenameGroup = (tag: string) => {
    setRenamingTag(tag);
    setRenameValue(tag);
  };

  const cancelRenameGroup = () => {
    setRenamingTag(null);
    setRenameValue('');
  };

  const commitRenameGroup = async () => {
    if (!renamingTag) return;
    const next = renameValue.trim();
    if (!next || next === renamingTag) {
      cancelRenameGroup();
      return;
    }
    const items = responses.filter((item) => (item.tag || DEFAULT_TAG) === renamingTag);
    await Promise.all(items.map((item) => updateShortcut.mutateAsync({
      responseId: item.id,
      payload: {
        short_code: item.short_code,
        title: item.title,
        content: item.content,
        tag: next,
      },
    })));
    cancelRenameGroup();
    toast.success('Tag renamed');
  };

  const handleDeleteGroup = async (tag: string) => {
    const items = tag === '__all__'
      ? responses
      : responses.filter((item) => (item.tag || DEFAULT_TAG) === tag);
    await Promise.all(items.map((item) => deleteShortcut.mutateAsync(item.id)));
    setPendingGroupDelete(null);
    toast.success('Shortcuts deleted');
  };

  return (
    <div className="space-y-6">
      <section className="rounded-lg border bg-card">
        <div className="flex flex-col gap-3 border-b p-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 className="text-sm font-semibold">Message shortcuts</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              {responses.length} saved {responses.length === 1 ? 'reply' : 'replies'} · insert from the composer with a bang trigger.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <div className="relative w-full md:w-72">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search shortcuts..."
                className="pl-8 pr-8"
              />
              {search ? (
                <button
                  type="button"
                  onClick={() => setSearch('')}
                  aria-label="Clear search"
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  <Cancel01Icon className="h-3.5 w-3.5" />
                </button>
              ) : null}
            </div>
            <Button onClick={() => setAddOpen(true)}>
              <PlusSignIcon className="h-4 w-4" />
              Add shortcut
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="Shortcut bulk actions">
                  <MoreHorizontalIcon className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive"
                  disabled={responses.length === 0}
                  onSelect={() => setPendingGroupDelete('__all__')}
                >
                  <Delete01Icon className="h-4 w-4" />
                  Delete all shortcuts
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <div className="divide-y">
          {isLoading ? (
            <div className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
              <Loading01Icon className="h-4 w-4 animate-spin" />
              Loading shortcuts...
            </div>
          ) : responses.length === 0 ? (
            <div className="flex flex-col items-center gap-2 p-10 text-center">
              <div className="rounded-full bg-muted/50 p-3">
                <StickyNote01Icon className="h-6 w-6 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium">No shortcuts yet</p>
              <p className="max-w-xs text-sm text-muted-foreground">
                Add your first saved reply above to insert it from the composer with a bang trigger.
              </p>
            </div>
          ) : groups.length === 0 ? (
            <div className="flex flex-col items-center gap-2 p-10 text-center">
              <p className="text-sm font-medium">No matches for &ldquo;{search}&rdquo;</p>
              <Button variant="ghost" size="sm" onClick={() => setSearch('')}>Clear search</Button>
            </div>
          ) : groups.map((group) => {
            const isCollapsed = collapsed[group.tag] ?? false;
            return (
              <div key={group.tag}>
                <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/25 px-4 py-3">
                  <div className="flex flex-1 items-center gap-2">
                    {renamingTag === group.tag ? (
                      <>
                        <Input
                          autoFocus
                          value={renameValue}
                          onChange={(event) => setRenameValue(event.target.value)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') void commitRenameGroup();
                            if (event.key === 'Escape') cancelRenameGroup();
                          }}
                          className="h-8 max-w-[220px]"
                          aria-label="Tag name"
                        />
                        <Button variant="ghost" size="icon" onClick={() => void commitRenameGroup()} aria-label="Save tag name">
                          <TickDouble01Icon className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={cancelRenameGroup} aria-label="Cancel rename">
                          <Cancel01Icon className="h-4 w-4" />
                        </Button>
                      </>
                    ) : (
                      <>
                        <h4 className="text-sm font-semibold">{group.tag === DEFAULT_TAG ? 'Others (default)' : group.tag}</h4>
                        <Badge variant="secondary">{group.items.length} shortcuts</Badge>
                      </>
                    )}
                  </div>
                  <div className="flex items-center gap-1">
                    {renamingTag === group.tag ? null : (
                      <Button variant="ghost" size="icon" onClick={() => startRenameGroup(group.tag)} aria-label="Rename tag">
                        <PencilEdit01Icon className="h-4 w-4" />
                      </Button>
                    )}
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" aria-label="Group actions">
                          <MoreHorizontalIcon className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onSelect={() => setPendingGroupDelete(group.tag)}
                        >
                          <Delete01Icon className="h-4 w-4" />
                          Delete group shortcuts
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setCollapsed((prev) => ({ ...prev, [group.tag]: !isCollapsed }))}
                    >
                      {isCollapsed ? <ArrowDown01Icon className="h-4 w-4" /> : <ArrowUp01Icon className="h-4 w-4" />}
                      {isCollapsed ? 'Show' : 'Hide'}
                    </Button>
                  </div>
                </div>
                {!isCollapsed ? (
                  <div className="divide-y">
                    {group.items.map((item) => {
                      const isEditing = editingId === item.id;
                      return (
                        <div key={item.id} className={cn('p-4', isEditing && 'bg-muted/20')}>
                          {isEditing ? (
                            <ShortcutForm
                              state={editingForm}
                              workspaceId={workspaceId}
                              tags={tags}
                              existing={responses}
                              editingId={item.id}
                              pending={updateShortcut.isPending}
                              submitLabel="Save Shortcut"
                              onChange={setEditingForm}
                              onSubmit={(markdown) => void handleUpdate(item.id, markdown)}
                              onCancel={() => setEditingId(null)}
                            />
                          ) : (
                            <div className="flex items-center justify-between gap-4">
                              <div className="min-w-0">
                                <div className="font-mono text-sm font-semibold">{item.short_code}</div>
                                <div className="mt-1 truncate text-sm text-muted-foreground">{stripHTML(item.content) || item.title}</div>
                              </div>
                              <div className="flex shrink-0 items-center gap-1">
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  onClick={() => {
                                    setEditingId(item.id);
                                    setEditingForm(formFromResponse(item));
                                  }}
                                  aria-label="Edit shortcut"
                                >
                                  <PencilEdit01Icon className="h-4 w-4" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="text-destructive hover:text-destructive"
                                  onClick={() => setPendingDelete(item)}
                                  aria-label="Delete shortcut"
                                >
                                  <Delete01Icon className="h-4 w-4" />
                                </Button>
                              </div>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </section>

      <AddShortcutDialog
        open={addOpen}
        workspaceId={workspaceId}
        onOpenChange={setAddOpen}
      />

      <AlertDialog open={!!pendingDelete} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete shortcut?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes {pendingDelete?.short_code} for everyone in the workspace.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (pendingDelete) void deleteShortcut.mutateAsync(pendingDelete.id).then(() => {
                  setPendingDelete(null);
                  toast.success('Shortcut deleted');
                });
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={!!pendingGroupDelete} onOpenChange={(open) => !open && setPendingGroupDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete shortcuts?</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently removes {pendingGroupDelete === '__all__' ? 'all shortcuts' : `all shortcuts in ${pendingGroupDelete}`}.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (pendingGroupDelete) void handleDeleteGroup(pendingGroupDelete);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
