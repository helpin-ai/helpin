import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import Link from '@tiptap/extension-link';
import {
  Bold,
  Code2,
  Heading2,
  ImageIcon,
  Italic,
  Link2,
  List,
  ListOrdered,
  Quote,
  Strikethrough,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef } from 'react';
import type { EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { uploadEditorImage } from '@/hooks/useEditorImageUpload';
import { ResizableImageExtension } from './resizable-image-extension';

export type { EditorUploadConfig };

interface TiptapEditorProps {
  content: string;
  onChange: (content: string) => void;
  placeholder?: string;
  className?: string;
  uploadConfig?: EditorUploadConfig;
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
      className={`inline-flex h-7 w-7 items-center justify-center rounded-sm transition-colors cursor-pointer
        ${active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
      `}
    >
      {children}
    </button>
  );
}

export function TiptapEditor({ content, onChange, placeholder = "Start writing...", className, uploadConfig }: TiptapEditorProps) {
  const uploadConfigRef = useRef(uploadConfig);
  uploadConfigRef.current = uploadConfig;

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
      try {
        const publicUrl = await uploadEditorImage(file, uploadConfigRef.current!);

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
                src: publicUrl,
                title: null,
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
      }
    },
    [],
  );

  const extensions = useMemo(() => {
    const exts = [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
      Placeholder.configure({ placeholder }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { class: 'text-primary underline cursor-pointer' },
      }),
    ];
    if (uploadConfig) {
      exts.push(ResizableImageExtension as typeof exts[number]);
    }
    return exts;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [placeholder, !!uploadConfig]);

  const editor = useEditor({
    extensions,
    content,
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[120px] px-4 py-3',
      },
      handlePaste: (view, event) => {
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
      handleDrop: (view, event, _slice, moved) => {
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
    },
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML());
    },
  });

  const editorRef = useRef(editor);
  editorRef.current = editor;

  // Sync external content changes (e.g. form reset)
  useEffect(() => {
    if (!editor) return;
    const current = editor.getHTML();
    if (content === '' && current !== '<p></p>' && current !== '') {
      editor.commands.clearContent();
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
      <div className="flex flex-wrap items-center gap-0.5 border-b border-border/60 px-2 py-1.5">
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBold().run()}
          active={editor.isActive('bold')}
        >
          <Bold className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleItalic().run()}
          active={editor.isActive('italic')}
        >
          <Italic className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleStrike().run()}
          active={editor.isActive('strike')}
        >
          <Strikethrough className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCode().run()}
          active={editor.isActive('code')}
        >
          <Code2 className="h-3.5 w-3.5" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          active={editor.isActive('heading', { level: 2 })}
        >
          <Heading2 className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          active={editor.isActive('bulletList')}
        >
          <List className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          active={editor.isActive('orderedList')}
        >
          <ListOrdered className="h-3.5 w-3.5" />
        </ToolbarButton>
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          active={editor.isActive('blockquote')}
        >
          <Quote className="h-3.5 w-3.5" />
        </ToolbarButton>

        <div className="mx-1 h-4 w-px bg-border/60" />

        <ToolbarButton onClick={addLink} active={editor.isActive('link')}>
          <Link2 className="h-3.5 w-3.5" />
        </ToolbarButton>
        {uploadConfig && (
          <ToolbarButton onClick={addImage}>
            <ImageIcon className="h-3.5 w-3.5" />
          </ToolbarButton>
        )}
      </div>

      {/* Editor area */}
      <EditorContent editor={editor} className="min-h-0 flex-1 overflow-y-auto" />
    </div>
  );
}
