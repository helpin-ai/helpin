import { useEffect, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import {
  QuietConversationComposer,
  QuietComposerEditorSurface,
  QuietComposerToolbar,
  QuietSelect,
} from "@/components/design-system/quiet";
import { EmojiPicker } from "@/components/support/EmojiPicker";
import { LinkInsertModal } from "@/components/support/LinkInsertModal";

const variables = [
  ["first_name|there", "First name"],
  ["full_name", "Full name"],
  ["company|your team", "Company"],
  ["deal_name", "Deal name"],
  ["sender_email", "Sender email"],
];
export function EmailContentEditor({
  workspaceId,
  value,
  onChange,
  disabled = false,
  tools,
}: {
  workspaceId: string;
  value: string;
  onChange: (html: string) => void;
  disabled?: boolean;
  tools?: React.ReactNode;
}) {
  const [focused, setFocused] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: false,
        codeBlock: false,
        link: { openOnClick: false },
      }),
      Placeholder.configure({ placeholder: "Write your email…" }),
    ],
    content: value,
    editable: !disabled,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        class:
          "rich-text-soft prose prose-sm dark:prose-invert min-h-44 max-h-80 max-w-none overflow-y-auto px-3 py-3 focus:outline-none",
        "aria-label": "Email message",
      },
    },
    onUpdate: ({ editor }) => {
      if (editor.isFocused) onChange(editor.getHTML());
    },
    onFocus: () => setFocused(true),
    onBlur: () => setFocused(false),
  });
  useEffect(() => {
    editor?.setEditable(!disabled);
  }, [editor, disabled]);
  useEffect(() => {
    if (editor && editor.getHTML() !== value)
      editor.commands.setContent(value, { emitUpdate: false });
  }, [editor, value]);
  if (!editor) return null;
  return (
    <QuietConversationComposer focused={focused} className="min-w-0">
      <div className="flex flex-wrap items-center gap-2 px-3 pt-2">
        <QuietSelect
          label="Insert variable"
          value="personalize"
          options={[
            { value: "personalize", label: "Personalize", disabled: true },
            ...variables.map(([value, label]) => ({ value, label })),
          ]}
          onChange={(value) =>
            editor.chain().focus().insertContent(`{{${value}}}`).run()
          }
          disabled={disabled}
        />
        {tools}
      </div>
      <QuietComposerEditorSurface>
        <EditorContent editor={editor} />
      </QuietComposerEditorSurface>
      <QuietComposerToolbar
        editor={editor}
        emoji={
          <EmojiPicker
            side="top"
            onEmojiSelect={(emoji) =>
              editor.chain().focus().insertContent(emoji).run()
            }
          />
        }
        onLink={() => setLinkOpen(true)}
        showShortcut={false}
      />
      <LinkInsertModal
        workspaceId={workspaceId}
        open={linkOpen}
        onOpenChange={setLinkOpen}
        onInsert={(label, url) =>
          editor
            .chain()
            .focus()
            .insertContent({
              type: "text",
              text: label,
              marks: [{ type: "link", attrs: { href: url } }],
            })
            .run()
        }
      />
    </QuietConversationComposer>
  );
}
