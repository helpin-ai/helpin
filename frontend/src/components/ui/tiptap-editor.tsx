import { useEditor, EditorContent, type Editor } from '@tiptap/react';
import { Mark, mergeAttributes } from '@tiptap/core';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import UnderlineExtension from '@tiptap/extension-underline';
import { Table } from '@tiptap/extension-table';
import { TableRow } from '@tiptap/extension-table-row';
import { TableHeader } from '@tiptap/extension-table-header';
import { TableCell } from '@tiptap/extension-table-cell';
import { TaskList } from '@tiptap/extension-task-list';
import { TaskItem } from '@tiptap/extension-task-item';
import { Markdown } from 'tiptap-markdown';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { diffRemovedInlineAttachmentIds, normalizeInlineAttachmentImageSrcs } from '@/components/pm/editorImageAttachments';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  AttachmentIcon,
  CheckListIcon,
  File01Icon,
  Heading02Icon,
  Heading03Icon,
  Image01Icon,
  LayoutTable01Icon,
  Link01Icon,
  Menu01Icon,
  MinusSignIcon,
  PaintBoardIcon,
  PlusSignIcon,
  QuoteDownIcon,
  SmileIcon,
  SourceCodeIcon,
  SourceCodeIcon as FileCode2Icon,
  TextBoldIcon,
  TextItalicIcon,
  TextStrikethroughIcon,
  TextUnderlineIcon,
} from '@/lib/icons';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { uploadEditorFile, uploadEditorImage } from '@/hooks/useEditorImageUpload';
import type { WorkspaceTeam, AssignableMember } from '@/lib/types';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { ResizableImageExtension } from './resizable-image-extension';
import {
  getMemberMentionHandle,
  getMentionSuggestions,
  type MentionSuggestionItem,
} from '@/components/pm/mentionSuggestions';

export type { EditorUploadConfig };

const TEXT_COLORS = [
  { label: 'Default', value: null, swatch: 'var(--foreground)' },
  { label: 'Gray', value: '#64748b', swatch: '#64748b' },
  { label: 'Red', value: '#dc2626', swatch: '#dc2626' },
  { label: 'Orange', value: '#ea580c', swatch: '#ea580c' },
  { label: 'Yellow', value: '#ca8a04', swatch: '#ca8a04' },
  { label: 'Green', value: '#16a34a', swatch: '#16a34a' },
  { label: 'Blue', value: '#2563eb', swatch: '#2563eb' },
  { label: 'Purple', value: '#9333ea', swatch: '#9333ea' },
];

const COMMON_EMOJIS = ['😀', '😊', '🙌', '👍', '🎉', '✅', '🙏', '💡', '🚀', '❤️', '👋', '✨'];

const TextColor = Mark.create({
  name: 'textColor',
  addAttributes() {
    return {
      color: {
        default: null,
        parseHTML: (element) => element.style.color || null,
        renderHTML: (attributes) => attributes.color ? { style: `color: ${attributes.color}` } : {},
      },
    };
  },
  parseHTML() {
    return [{ style: 'color' }];
  },
  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes), 0];
  },
});

type TableChain = ReturnType<Editor['chain']> & {
  insertTable: (options: { rows: number; cols: number; withHeaderRow?: boolean }) => ReturnType<Editor['chain']>;
};

function escapeHTML(value: string) {
  const element = document.createElement('div');
  element.textContent = value;
  return element.innerHTML;
}

interface TiptapEditorProps {
  content: string;
  onChange: (content: string) => void;
  placeholder?: string;
  className?: string;
  uploadConfig?: EditorUploadConfig;
  onUploadStateChange?: (pendingUploads: number) => void;
  onUploadReady?: (upload: ((files: FileList | File[], insertPos?: number) => Promise<void>) | null) => void;
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  members?: AssignableMember[];
  onEditorReady?: (editor: Editor | null) => void;
}

function ToolbarButton({
  onClick,
  active = false,
  title,
  children,
}: {
  onClick: () => void;
  active?: boolean;
  title?: string;
  children: React.ReactNode;
}) {
  const btn = (
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

  if (!title) return btn;

  return (
    <Tooltip>
      <TooltipTrigger asChild>{btn}</TooltipTrigger>
      <TooltipContent side="bottom" className="text-xs">{title}</TooltipContent>
    </Tooltip>
  );
}

export function TiptapEditor({ content, onChange, placeholder = "Start writing...", className, uploadConfig, onUploadStateChange, onUploadReady, teams = [], members = [], onEditorReady }: TiptapEditorProps) {
  const uploadConfigRef = useRef(uploadConfig);
  uploadConfigRef.current = uploadConfig;
  const onUploadStateChangeRef = useRef(onUploadStateChange);
  onUploadStateChangeRef.current = onUploadStateChange;
  const pendingUploadsRef = useRef(0);
  const normalizedContent = useMemo(() => normalizeInlineAttachmentImageSrcs(content), [content]);
  const currentHtmlRef = useRef(normalizedContent);
  currentHtmlRef.current = normalizedContent;
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
  const uploadsEnabled = Boolean(uploadConfig);

  const focusAtInsertPos = useCallback((editorInstance: Editor, insertPos?: number) => {
    const chain = editorInstance.chain().focus();
    if (typeof insertPos !== 'number') return chain;
    const safePos = Math.max(0, Math.min(insertPos, editorInstance.state.doc.content.size));
    return chain.setTextSelection(safePos);
  }, []);

  const handleImageUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>, insertPos?: number) => {
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
      focusAtInsertPos(editorInstance, insertPos)
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
    [focusAtInsertPos],
  );

  const handleFileUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>, insertPos?: number) => {
      if (!editorInstance || !uploadConfigRef.current) return;
      pendingUploadsRef.current += 1;
      onUploadStateChangeRef.current?.(pendingUploadsRef.current);
      try {
        const upload = await uploadEditorFile(file, uploadConfigRef.current);
        const fileName = escapeHTML(file.name || 'Attachment');
        const href = escapeHTML(upload.publicUrl);
        focusAtInsertPos(editorInstance, insertPos)
          .insertContent(`<a href="${href}" target="_blank" rel="noopener noreferrer">${fileName}</a>`)
          .run();
      } catch {
        // Keep the editor content unchanged when a file upload fails.
      } finally {
        pendingUploadsRef.current = Math.max(0, pendingUploadsRef.current - 1);
        onUploadStateChangeRef.current?.(pendingUploadsRef.current);
      }
    },
    [focusAtInsertPos],
  );

  const handleFilesUpload = useCallback(
    async (files: FileList | File[], insertPos?: number) => {
      const editorInstance = editorRef.current;
      if (!editorInstance || !uploadConfigRef.current) return;

      if (typeof insertPos === 'number') {
        focusAtInsertPos(editorInstance, insertPos).run();
      }
      for (const file of Array.from(files)) {
        if (file.type.startsWith('image/')) {
          await handleImageUpload(file, editorInstance);
        } else {
          await handleFileUpload(file, editorInstance);
        }
      }
    },
    [focusAtInsertPos, handleFileUpload, handleImageUpload],
  );

  const extensions = useMemo(() => {
    const exts = [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
        link: {
          openOnClick: false,
          HTMLAttributes: { class: 'text-blue-600 dark:text-blue-400 underline cursor-pointer', target: '_blank', rel: 'noopener noreferrer' },
        },
      }),
      UnderlineExtension,
      TextColor,
      Placeholder.configure({
        placeholder,
        showOnlyCurrent: false,
        emptyNodeClass: 'is-empty',
        emptyEditorClass: 'is-editor-empty',
      }),
      Table.configure({ resizable: false }),
      TableRow,
      TableHeader,
      TableCell,
      TaskList,
      TaskItem.configure({ nested: true }),
      Markdown.configure({
        html: true,
        tightLists: true,
        bulletListMarker: '-',
        transformPastedText: true,
        transformCopiedText: false,
      }),
      MentionHighlight.configure({
        validHandles: () => {
          const handles = new Set<string>();
          for (const m of membersRef.current) {
            const handle = getMemberMentionHandle(m);
            if (handle) handles.add(handle.toLowerCase());
          }
          for (const t of teamsRef.current) {
            if (t.handle) handles.add(t.handle.toLowerCase());
          }
          return handles;
        },
      }),
    ];
    if (uploadConfig) {
      exts.push(ResizableImageExtension.configure({ enableCaption: false }) as typeof exts[number]);
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
    content: normalizedContent,
    editorProps: {
      attributes: {
        class: 'tiptap prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[120px] px-4 py-3',
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

        event.preventDefault();
        const dropPos = editorRef.current?.view.posAtCoords({
          left: event.clientX,
          top: event.clientY,
        })?.pos;
        void handleFilesUpload(files, dropPos);
        return true;
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

  useEffect(() => {
    onEditorReady?.(editor);
    return () => onEditorReady?.(null);
  }, [editor, onEditorReady]);

  useEffect(() => {
    if (!uploadsEnabled || !editor) {
      onUploadReady?.(null);
      return;
    }
    onUploadReady?.(handleFilesUpload);
    return () => onUploadReady?.(null);
  }, [editor, handleFilesUpload, onUploadReady, uploadsEnabled]);

  // Sync external content changes (e.g. form reset, template apply)
  useEffect(() => {
    if (!editor) return;
    const current = editor.getHTML();
    if (normalizedContent === '' && current !== '<p></p>' && current !== '') {
      editor.commands.clearContent();
    } else if (normalizedContent !== '' && normalizedContent !== current) {
      editor.commands.setContent(normalizedContent);
    }
  }, [normalizedContent, editor]);

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

  const addAttachment = () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.onchange = () => {
      const file = input.files?.[0];
      if (file) {
        handleFileUpload(file, editor);
      }
    };
    input.click();
  };

  const insertTable = () => {
    (editor.chain().focus() as TableChain)
      .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
      .run();
  };

  const setTextColor = (color: string | null) => {
    const chain = editor.chain().focus();
    if (color) {
      chain.setMark('textColor', { color }).run();
    } else {
      chain.unsetMark('textColor').run();
    }
  };

  const insertEmoji = (emoji: string) => {
    editor.chain().focus().insertContent(emoji).run();
  };

  return (
    <div className={`overflow-hidden rounded-2xl border border-transparent bg-input/50 ${className ?? ''}`}>
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-1 border-b border-border/40 px-2.5 py-2">
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBold().run()}
          active={editor.isActive('bold')}
          title="Bold"
        >
          <TextBoldIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleItalic().run()}
          active={editor.isActive('italic')}
          title="Italic"
        >
          <TextItalicIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleUnderline().run()}
          active={editor.isActive('underline')}
          title="Underline"
        >
          <TextUnderlineIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleStrike().run()}
          active={editor.isActive('strike')}
          title="Strikethrough"
        >
          <TextStrikethroughIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCode().run()}
          active={editor.isActive('code')}
          title="Inline code"
        >
          <SourceCodeIcon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          active={editor.isActive('heading', { level: 2 })}
          title="Heading 2"
        >
          <Heading02Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          active={editor.isActive('heading', { level: 3 })}
          title="Heading 3"
        >
          <Heading03Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          active={editor.isActive('bulletList')}
          title="Bullet list"
        >
          <Menu01Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          active={editor.isActive('orderedList')}
          title="Numbered list"
        >
          <CheckListIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          active={editor.isActive('blockquote')}
          title="Quote"
        >
          <QuoteDownIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          active={editor.isActive('codeBlock')}
          title="Code block"
        >
          <FileCode2Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
          title="Horizontal rule"
        >
          <MinusSignIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton onClick={insertTable} title="Table">
          <LayoutTable01Icon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              title="Text color"
            >
              <PaintBoardIcon className="h-4 w-4" />
            </button>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-48 gap-2 p-2">
            <div className="grid grid-cols-4 gap-1">
              {TEXT_COLORS.map((color) => (
                <button
                  key={color.label}
                  type="button"
                  onClick={() => setTextColor(color.value)}
                  className="flex h-8 items-center justify-center rounded-md border border-border/60 hover:bg-accent"
                  title={color.label}
                >
                  <span
                    className="h-4 w-4 rounded-full border border-border"
                    style={{ backgroundColor: color.swatch }}
                  />
                </button>
              ))}
            </div>
          </PopoverContent>
        </Popover>
        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              title="Emoji"
            >
              <SmileIcon className="h-4 w-4" />
            </button>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-56 p-2">
            <div className="grid grid-cols-6 gap-1">
              {COMMON_EMOJIS.map((emoji) => (
                <button
                  key={emoji}
                  type="button"
                  onClick={() => insertEmoji(emoji)}
                  className="flex h-8 items-center justify-center rounded-md text-base hover:bg-accent"
                >
                  {emoji}
                </button>
              ))}
            </div>
          </PopoverContent>
        </Popover>
        <ToolbarButton onClick={addLink} active={editor.isActive('link')} title="Link">
          <Link01Icon className="h-4 w-4" />
        </ToolbarButton>
        {uploadConfig && (
          <>
            <ToolbarButton onClick={addImage} title="Image">
              <Image01Icon className="h-4 w-4" />
            </ToolbarButton>
            <ToolbarButton onClick={addAttachment} title="Attachment">
              <AttachmentIcon className="h-4 w-4" />
            </ToolbarButton>
          </>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="ghost" size="icon-sm" title="Insert">
              <PlusSignIcon className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuItem onSelect={insertTable}>
              <LayoutTable01Icon className="h-4 w-4" />
              Table
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => editor.chain().focus().setHorizontalRule().run()}>
              <MinusSignIcon className="h-4 w-4" />
              Divider
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => insertEmoji('👋')}>
              <SmileIcon className="h-4 w-4" />
              Emoji
            </DropdownMenuItem>
            {uploadConfig ? (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={addImage}>
                  <Image01Icon className="h-4 w-4" />
                  Image
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={addAttachment}>
                  <File01Icon className="h-4 w-4" />
                  Attachment
                </DropdownMenuItem>
              </>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Editor area */}
      <EditorContent editor={editor} className="min-h-0 flex-1 overflow-y-auto" />
      {mentionState && mentionState.items.length > 0 ? (
        <div className="border-t border-border/40 p-1.5">
          <p className="px-2 pb-1 pt-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">
            Suggestions
          </p>
          <div className="max-h-[200px] overflow-y-auto">
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
        </div>
      ) : null}
    </div>
  );
}
