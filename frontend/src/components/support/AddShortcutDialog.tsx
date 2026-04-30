import { useEffect, useMemo, useRef, useState } from 'react';
import type { Editor } from '@tiptap/react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { useCannedResponses, useCreateCannedResponse } from '@/hooks/queries/useSupport';
import type { SupportCannedResponse } from '@/lib/pmTypes';
import { Loading01Icon, PlusSignIcon } from '@/lib/icons';

const DEFAULT_TAG = 'Others';
const BUILT_IN_TAGS = [DEFAULT_TAG, 'Support', 'Sales', 'Billing'];
const CUSTOM_TAG_VALUE = '__custom_tag__';

function stripHTML(value: string) {
  if (!value) return '';
  const doc = new DOMParser().parseFromString(value, 'text/html');
  return doc.body.textContent?.replace(/\s+/g, ' ').trim() ?? '';
}

function readEditorMarkdown(editor: Editor | null): string {
  if (!editor) return '';
  const storage = (editor.storage as { markdown?: { getMarkdown(): string } }).markdown;
  if (storage?.getMarkdown) return storage.getMarkdown().trim();
  return editor.getText().trim();
}

function validateShortcut(shortCode: string) {
  const value = shortCode.trim();
  if (!value) return 'Shortcut is required';
  if (!value.startsWith('!')) return 'Shortcut must start with !';
  if (/\s/.test(value)) return 'Shortcut cannot contain spaces';
  return null;
}

function buildTagOptions(responses: SupportCannedResponse[]) {
  return Array.from(new Set([...BUILT_IN_TAGS, ...responses.map((item) => item.tag || DEFAULT_TAG)]));
}

function seedShortCode(seed?: string) {
  if (!seed) return '';
  const trimmed = seed.trim();
  if (!trimmed) return '';
  return trimmed.startsWith('!') ? trimmed : `!${trimmed}`;
}

export interface AddShortcutDialogProps {
  open: boolean;
  workspaceId: string;
  /** Pre-fill the shortcode field. Useful when opened from the composer with a typed query. */
  seedShortCode?: string;
  onOpenChange: (open: boolean) => void;
  onCreated?: (shortcut: SupportCannedResponse) => void;
}

export function AddShortcutDialog({ open, workspaceId, seedShortCode: seed, onOpenChange, onCreated }: AddShortcutDialogProps) {
  const { data: responses = [] } = useCannedResponses(workspaceId);
  const createShortcut = useCreateCannedResponse(workspaceId);

  const [shortCode, setShortCode] = useState('');
  const [title, setTitle] = useState('');
  const [tag, setTag] = useState(DEFAULT_TAG);
  const [customTag, setCustomTag] = useState('');
  const [content, setContent] = useState('');
  const [pendingUploads, setPendingUploads] = useState(0);
  const editorRef = useRef<Editor | null>(null);

  useEffect(() => {
    if (!open) return;
    setShortCode(seedShortCode(seed));
    setTitle('');
    setTag(DEFAULT_TAG);
    setCustomTag('');
    setContent('');
    setPendingUploads(0);
  }, [open, seed]);

  const tags = useMemo(() => buildTagOptions(responses), [responses]);
  const shortcutError = validateShortcut(shortCode);
  const normalizedCode = shortCode.trim();
  const duplicate = responses.some((item) => item.short_code === normalizedCode);
  const customTagMissing = tag === CUSTOM_TAG_VALUE && !customTag.trim();
  const contentText = stripHTML(content);
  const canSubmit = !shortcutError && !duplicate && !customTagMissing && contentText.length > 0 && !createShortcut.isPending && pendingUploads === 0;

  const submit = async () => {
    const resolvedTag = tag === CUSTOM_TAG_VALUE ? (customTag.trim() || DEFAULT_TAG) : tag;
    // Save markdown so the composer's Markdown extension expands it correctly on insert.
    // (insertContent on a string is parsed as markdown when html: false on the extension.)
    const markdownContent = readEditorMarkdown(editorRef.current) || stripHTML(content);
    try {
      const created = await createShortcut.mutateAsync({
        short_code: normalizedCode,
        title: title.trim() || normalizedCode,
        content: markdownContent,
        tag: resolvedTag,
      });
      toast.success('Shortcut added');
      onCreated?.(created);
      onOpenChange(false);
    } catch {}
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Add shortcut</DialogTitle>
          <DialogDescription>
            Save this reply so you can insert it from the composer with a bang trigger.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="shortcut-code">Shortcut</Label>
              <Input
                id="shortcut-code"
                value={shortCode}
                onChange={(event) => setShortCode(event.target.value)}
                placeholder="!bang"
                className="font-mono"
                autoFocus
              />
              {shortcutError ? (
                <p className="text-xs text-destructive">{shortcutError}</p>
              ) : duplicate ? (
                <p className="text-xs text-destructive">That shortcut already exists.</p>
              ) : (
                <p className="text-xs text-muted-foreground">Starts with <span className="font-mono">!</span>, no spaces.</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="shortcut-title">Name <span className="text-muted-foreground">(optional)</span></Label>
              <Input
                id="shortcut-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Greeting"
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Tag</Label>
            <Select value={tag} onValueChange={setTag}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {tags.map((t) => (
                  <SelectItem key={t} value={t}>
                    {t === DEFAULT_TAG ? 'Others (default)' : t}
                  </SelectItem>
                ))}
                <SelectItem value={CUSTOM_TAG_VALUE}>Create new tag</SelectItem>
              </SelectContent>
            </Select>
            {tag === CUSTOM_TAG_VALUE ? (
              <Input
                value={customTag}
                onChange={(event) => setCustomTag(event.target.value)}
                placeholder="Tag name"
              />
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label>Message</Label>
            <TiptapEditor
              content={content}
              onChange={setContent}
              placeholder="Write the saved reply..."
              className="rounded-lg border-border bg-background"
              uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
              onUploadStateChange={setPendingUploads}
              onEditorReady={(editor) => { editorRef.current = editor; }}
            />
            {pendingUploads > 0 ? <p className="text-xs text-muted-foreground">Uploading attachments...</p> : null}
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="button" disabled={!canSubmit} onClick={() => void submit()}>
            {createShortcut.isPending ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlusSignIcon className="h-4 w-4" />}
            Add shortcut
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
