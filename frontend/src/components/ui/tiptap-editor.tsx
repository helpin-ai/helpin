import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { diffRemovedInlineAttachmentIds } from '@/components/pm/editorImageAttachments';
import {
  Bold,
  Code2,
  FileCode2,
  Heading2,
  Heading3,
  ImageIcon,
  Italic,
  Link2,
  List,
  ListOrdered,
  Minus,
  Quote,
  Strikethrough,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { uploadEditorImage } from '@/hooks/useEditorImageUpload';
import type { WorkspaceTeam, AssignableMember } from '@/lib/types';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { ResizableImageExtension } from './resizable-image-extension';
import {
  getMentionSuggestions,
  type MentionSuggestionItem,
} from '@/components/pm/mentionSuggestions';

export type { EditorUploadConfig };

interface TiptapEditorProps {
  content: string;
  onChange: (content: string) => void;
  placeholder?: string;
  className?: string;
  uploadConfig?: EditorUploadConfig;
  onUploadStateChange?: (pendingUploads: number) => void;
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  members?: AssignableMember[];
}

function ToolbarButton({
  onClick,
  active = false,
  children,
}: {
  onClick: () => void;
  active?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex h-8 w-8 items-center justify-center rounded-md transition-colors cursor-pointer
        ${active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
      `}
    >
      {children}
    </button>
  );
}

export function TiptapEditor({ content, onChange, placeholder = "Start writing...", className, uploadConfig, onUploadStateChange, teams = [], members = [] }: TiptapEditorProps) {
  const uploadConfigRef = useRef(uploadConfig);
  uploadConfigRef.current = uploadConfig;
  const onUploadStateChangeRef = useRef(onUploadStateChange);
  onUploadStateChangeRef.current = onUploadStateChange;
  const pendingUploadsRef = useRef(0);
  const currentHtmlRef = useRef(content);
  currentHtmlRef.current = content;
  const [mentionState, setMentionState] = useState<{
    from: number;
    to: number;
    items: MentionSuggestionItem[];
    selectedIndex: number;
  } | null>(null);
  const mentionStateRef = useRef(mentionState);
  mentionStateRef.current = mentionState;
  const teamsRef = useRef(teams);
  teamsRef.current = teams;
  const membersRef = useRef(members);
  membersRef.current = members;
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const handleImageUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>) => {
      if (!editorInstance || !uploadConfigRef.current) return;
      if (!file.type.startsWith('image/')) return;

      const uploadId = `img-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;

      // Read file as data URI for instant preview
      const dataUri = await new Promise<string>((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.readAsDataURL(file);
      });

      // Insert resizable image with data URI immediately
      editorInstance
        .chain()
        .focus()
        .setResizableImage({ src: dataUri, alt: file.name, title: uploadId })
        .run();

      // Upload in background
      pendingUploadsRef.current += 1;
      onUploadStateChangeRef.current?.(pendingUploadsRef.current);
      try {
        const upload = await uploadEditorImage(file, uploadConfigRef.current!);

        // Replace the data URI with the permanent public URL
        const { doc } = editorInstance.state;
        let targetPos: number | null = null;
        doc.descendants((node, pos) => {
          if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
            targetPos = pos;
            return false;
          }
        });

        if (targetPos !== null) {
          const node = doc.nodeAt(targetPos);
          if (node) {
            editorInstance.view.dispatch(
              editorInstance.state.tr.setNodeMarkup(targetPos, undefined, {
                ...node.attrs,
                src: upload.publicUrl,
                title: null,
                attachmentId: upload.attachmentId,
              }),
            );
          }
        }
      } catch {
        // On failure, remove the placeholder image
        const { doc } = editorInstance.state;
        let targetPos: number | null = null;
        doc.descendants((node, pos) => {
          if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
            targetPos = pos;
            return false;
          }
        });

        if (targetPos !== null) {
          const node = doc.nodeAt(targetPos);
          if (node) {
            editorInstance.view.dispatch(
              editorInstance.state.tr.delete(targetPos, targetPos + node.nodeSize),
            );
          }
        }
      } finally {
        pendingUploadsRef.current = Math.max(0, pendingUploadsRef.current - 1);
        onUploadStateChangeRef.current?.(pendingUploadsRef.current);
      }
    },
    [],
  );

  const extensions = useMemo(() => {
    const exts = [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
        link: {
          openOnClick: false,
          HTMLAttributes: { class: 'text-primary underline cursor-pointer' },
        },
      }),
      Placeholder.configure({ placeholder }),
      MentionHighlight,
    ];
    if (uploadConfig) {
      exts.push(ResizableImageExtension as typeof exts[number]);
    }
    return exts;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [placeholder, !!uploadConfig]);

  const cleanupDraftAttachments = useCallback(async (attachmentIds: string[]) => {
    const currentConfig = uploadConfigRef.current;
    if (!currentConfig || currentConfig.entityType !== 'editor_upload' || attachmentIds.length === 0) {
      return;
    }
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(currentConfig.workspaceId, attachmentId)),
    );
  }, []);

  const editor = useEditor({
    extensions,
    content,
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[120px] px-4 py-3',
      },
      handlePaste: (_view, event) => {
        if (!uploadConfigRef.current) return false;
        const items = event.clipboardData?.items;
        if (!items) return false;

        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault();
            const file = item.getAsFile();
            if (file) {
              handleImageUpload(file, editorRef.current);
            }
            return true;
          }
        }
        return false;
      },
      handleDrop: (_view, event, _slice, moved) => {
        if (!uploadConfigRef.current || moved) return false;
        const files = event.dataTransfer?.files;
        if (!files?.length) return false;

        for (const file of files) {
          if (file.type.startsWith('image/')) {
            event.preventDefault();
            handleImageUpload(file, editorRef.current);
            return true;
          }
        }
        return false;
      },
      handleKeyDown: (_view, event) => {
        const currentMention = mentionStateRef.current;
        if (!currentMention || currentMention.items.length === 0) return false;
        if (event.key === 'ArrowDown') {
          event.preventDefault();
          setMentionState({
            ...currentMention,
            selectedIndex: (currentMention.selectedIndex + 1) % currentMention.items.length,
          });
          return true;
        }
        if (event.key === 'ArrowUp') {
          event.preventDefault();
          setMentionState({
            ...currentMention,
            selectedIndex: (currentMention.selectedIndex - 1 + currentMention.items.length) % currentMention.items.length,
          });
          return true;
        }
        if (event.key === 'Enter' || event.key === 'Tab') {
          const selected = currentMention.items[currentMention.selectedIndex];
          if (!selected || !editorRef.current) return false;
          event.preventDefault();
          editorRef.current
            .chain()
            .focus()
            .insertContentAt({ from: currentMention.from, to: currentMention.to }, `@${selected.handle} `)
            .run();
          setMentionState(null);
          return true;
        }
        if (event.key === 'Escape') {
          event.preventDefault();
          setMentionState(null);
          return true;
        }
        return false;
      },
    },
  });

  const editorRef = useRef(editor);
  editorRef.current = editor;

  // Register update/blur handlers via useEffect — avoids TipTap v3 stale-closure
  // issue where onUpdate passed in useEditor options is only captured once.
  useEffect(() => {
    if (!editor) return;

    const handleUpdate = () => {
      const html = editor.getHTML();
      const removedDraftAttachmentIds = diffRemovedInlineAttachmentIds(currentHtmlRef.current, html);
      currentHtmlRef.current = html;
      onChangeRef.current(html);
      if (removedDraftAttachmentIds.length > 0) {
        void cleanupDraftAttachments(removedDraftAttachmentIds);
      }
      const currentTeams = teamsRef.current;
      const currentMembers = membersRef.current;
      if (currentTeams.length === 0 && currentMembers.length === 0) {
        setMentionState(null);
        return;
      }
      const { selection } = editor.state;
      if (!selection.empty) {
        setMentionState(null);
        return;
      }
      const textBefore = selection.$from.parent.textBetween(0, selection.$from.parentOffset, undefined, '\ufffc');
      const match = textBefore.match(/(?:^|\s)@([a-z0-9._-]*)$/i);
      if (!match) {
        setMentionState(null);
        return;
      }
      const query = match[1].toLowerCase();
      const items = getMentionSuggestions(query, currentMembers, currentTeams, 8);

      if (items.length === 0) {
        setMentionState(null);
        return;
      }
      setMentionState({
        from: selection.from - (query.length + 1),
        to: selection.from,
        items: items.slice(0, 8),
        selectedIndex: 0,
      });
    };

    const handleBlur = () => setMentionState(null);

    editor.on('update', handleUpdate);
    editor.on('blur', handleBlur);

    return () => {
      editor.off('update', handleUpdate);
      editor.off('blur', handleBlur);
    };
  }, [cleanupDraftAttachments, editor]);

  // Sync external content changes (e.g. form reset, template apply)
  useEffect(() => {
    if (!editor) return;
    const current = editor.getHTML();
    if (content === '' && current !== '<p></p>' && current !== '') {
      editor.commands.clearContent();
    } else if (content !== '' && content !== current) {
      editor.commands.setContent(content);
    }
  }, [content, editor]);

  if (!editor) return null;

  const addLink = () => {
    const url = window.prompt('URL');
    if (url) {
      editor.chain().focus().setLink({ href: url }).run();
    }
  };

  const addImage = () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.onchange = () => {
      const file = input.files?.[0];
      if (file) {
        handleImageUpload(file, editor);
      }
    };
    input.click();
  };

  return (
    <div className={`overflow-hidden rounded-lg border border-border/60 bg-background ${className ?? ''}`}>
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-1 border-b border-border/60 px-2.5 py-2">
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBold().run()}
          active={editor.isActive('bold')}
        >
          <Bold className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleItalic().run()}
          active={editor.isActive('italic')}
        >
          <Italic className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleStrike().run()}
          active={editor.isActive('strike')}
        >
          <Strikethrough className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCode().run()}
          active={editor.isActive('code')}
        >
          <Code2 className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          active={editor.isActive('heading', { level: 2 })}
        >
          <Heading2 className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          active={editor.isActive('heading', { level: 3 })}
        >
          <Heading3 className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          active={editor.isActive('bulletList')}
        >
          <List className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          active={editor.isActive('orderedList')}
        >
          <ListOrdered className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          active={editor.isActive('blockquote')}
        >
          <Quote className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          active={editor.isActive('codeBlock')}
        >
          <FileCode2 className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
        >
          <Minus className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <ToolbarButton onClick={addLink} active={editor.isActive('link')}>
          <Link2 className="h-4 w-4" />
        </ToolbarButton>
        {uploadConfig && (
          <ToolbarButton onClick={addImage}>
            <ImageIcon className="h-4 w-4" />
          </ToolbarButton>
        )}
      </div>

      {/* Editor area */}
      <EditorContent editor={editor} className="min-h-0 flex-1 overflow-y-auto" />
      {mentionState && mentionState.items.length > 0 ? (
        <div className="border-t border-border/60 bg-muted/40 px-2 py-2">
          <MentionSuggestionsList
            items={mentionState.items}
            selectedIndex={mentionState.selectedIndex}
            onSelect={(item) => {
              if (!editorRef.current) return;
              editorRef.current
                .chain()
                .focus()
                .insertContentAt({ from: mentionState.from, to: mentionState.to }, `@${item.handle} `)
                .run();
              setMentionState(null);
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
