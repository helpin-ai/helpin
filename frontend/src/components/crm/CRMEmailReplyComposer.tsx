import { useEffect, useMemo, useState } from 'react';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { toast } from 'sonner';
import { EmojiPicker } from '@/components/support/EmojiPicker';
import { LinkInsertModal } from '@/components/support/LinkInsertModal';
import {
  QuietComposerAITools,
  QuietComposerEditorSurface,
  QuietComposerToolbar,
  QuietConversationComposer,
  type ConversationRewriteOperation,
} from '@/components/design-system/quiet';
import { crmEmailService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';
import { useCRMEmailAttachments } from '@/hooks/useCRMEmailAttachments';
import { CRMEmailAttachmentStrip } from './CRMEmailAttachmentStrip';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';

export function CRMEmailReplyComposer({
  workspaceId,
  content,
  sending,
  onChange,
  onSubmit,
}: {
  workspaceId: string;
  content: string;
  sending: boolean;
  onChange: (content: string) => void;
  onSubmit: (attachments?: { draftId: string; attachmentIds: string[] }) => void | Promise<void>;
}) {
  const [focused, setFocused] = useState(false);
  const [rewriting, setRewriting] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState({ label: '', url: '' });
  const emailAttachments = useCRMEmailAttachments(workspaceId);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const submit = async () => {
    if (emailAttachments.uploading) return;
    await onSubmit({ draftId: emailAttachments.draftId, attachmentIds: emailAttachments.attachmentIds });
    emailAttachments.reset();
  };
  const extensions = useMemo(() => [
    StarterKit.configure({
      heading: false,
      codeBlock: false,
      horizontalRule: false,
      link: {
        openOnClick: false,
        autolink: true,
        linkOnPaste: true,
        HTMLAttributes: { target: '_blank', rel: 'noopener noreferrer nofollow' },
      },
    }),
    Placeholder.configure({ placeholder: 'Write a reply…' }),
  ], []);
  const editor = useEditor({
    extensions,
    content,
    editable: !sending,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        class: 'rich-text-soft pm-rich-text prose prose-sm dark:prose-invert max-w-none min-h-24 max-h-48 overflow-y-auto focus:outline-none',
      },
      handleKeyDown: (_view, event) => {
        if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
          event.preventDefault();
          void submit();
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: current }) => onChange(current.getHTML()),
    onFocus: () => setFocused(true),
    onBlur: () => setFocused(false),
  });

  useEffect(() => {
    editor?.setEditable(!sending);
  }, [editor, sending]);

  useEffect(() => {
    if (!editor || editor.getHTML() === content) return;
    editor.commands.setContent(content, { emitUpdate: false });
  }, [content, editor]);

  if (!editor) return null;
  const hasContent = Boolean(editor.getText().trim());
  const openLink = () => {
    const attrs = editor.getAttributes('link') as { href?: string };
    const { from, to, empty } = editor.state.selection;
    let label = '';
    if (editor.isActive('link')) {
      editor.chain().focus().extendMarkRange('link').run();
      label = editor.state.doc.textBetween(editor.state.selection.from, editor.state.selection.to, ' ');
    } else if (!empty) label = editor.state.doc.textBetween(from, to, ' ');
    setLinkInitial({ label, url: attrs.href ?? '' });
    setLinkOpen(true);
  };
  const insertLink = (label: string, url: string) => {
    if (editor.isActive('link')) editor.chain().focus().extendMarkRange('link').unsetLink().run();
    const { from, to, empty } = editor.state.selection;
    const node = { type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] };
    if (empty) editor.chain().focus().insertContent(node).run();
    else editor.chain().focus().insertContentAt({ from, to }, node).run();
  };
  const rewrite = async (operation: ConversationRewriteOperation) => {
    if (!hasContent || rewriting) return;
    setRewriting(true);
    try {
      const result = unwrap(await crmEmailService.rewriteDraft(workspaceId, editor.getHTML(), operation));
      editor.commands.setContent(result.content);
      editor.commands.focus('end');
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) setUpgradeReason(reason);
      else toast.error(error instanceof Error ? error.message : 'Reply could not be rewritten');
    } finally {
      setRewriting(false);
    }
  };

  return (
    <>
    <QuietConversationComposer focused={focused}>
      <div className="flex items-center px-3 pt-2">
        <QuietComposerAITools disabled={!hasContent} pending={rewriting} onSelect={rewrite} />
      </div>
      <QuietComposerEditorSurface><EditorContent editor={editor} /></QuietComposerEditorSurface>
      <CRMEmailAttachmentStrip attachments={emailAttachments.attachments} onRemove={(id) => void emailAttachments.remove(id)} />
      <QuietComposerToolbar
        editor={editor}
        emoji={<EmojiPicker side="top" onEmojiSelect={(emoji) => editor.chain().focus().insertContent(emoji).run()} />}
        onLink={openLink}
        onAttach={emailAttachments.pickFiles}
        onSubmit={() => void submit()}
        submitLabel="Send reply"
        submitDisabled={!hasContent || emailAttachments.uploading}
        submitting={sending}
      />
      <LinkInsertModal
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        initialLabel={linkInitial.label}
        initialUrl={linkInitial.url}
        onInsert={insertLink}
        onRemove={editor.isActive('link') ? () => editor.chain().focus().extendMarkRange('link').unsetLink().run() : undefined}
      />
    </QuietConversationComposer>
    <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(open) => { if (!open) setUpgradeReason(null); }} reason={upgradeReason} />
    </>
  );
}
