import type { Editor } from '@tiptap/core';
import {
  Heading2,
  Heading3,
  List,
  ListOrdered,
  Quote,
  Code2,
  MessageSquareWarning,
  Minus,
  Table,
  Image,
  type LucideIcon,
} from 'lucide-react';

export interface SlashCommand {
  title: string;
  description: string;
  icon: LucideIcon;
  iconColor?: string;
  action: (editor: Editor) => void;
  children?: SlashCommand[];
}

export const slashCommands: SlashCommand[] = [
  {
    title: 'Heading 2',
    description: 'Section heading',
    icon: Heading2,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    title: 'Heading 3',
    description: 'Small section heading',
    icon: Heading3,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    title: 'Bullet List',
    description: 'Unordered list',
    icon: List,
    action: (editor) => editor.chain().focus().toggleBulletList().run(),
  },
  {
    title: 'Numbered List',
    description: 'Ordered list',
    icon: ListOrdered,
    action: (editor) => editor.chain().focus().toggleOrderedList().run(),
  },
  {
    title: 'Blockquote',
    description: 'Quote or excerpt',
    icon: Quote,
    action: (editor) => editor.chain().focus().toggleBlockquote().run(),
  },
  {
    title: 'Code Block',
    description: 'Fenced code block',
    icon: Code2,
    action: (editor) => editor.chain().focus().toggleCodeBlock().run(),
  },
  {
    title: 'Divider',
    description: 'Horizontal rule',
    icon: Minus,
    action: (editor) => editor.chain().focus().setHorizontalRule().run(),
  },
  {
    title: 'Callout',
    description: 'Highlighted note or warning',
    icon: MessageSquareWarning,
    action: () => {}, // parent — opens submenu
    children: [
      {
        title: 'Blue callout',
        description: 'Informational note',
        icon: MessageSquareWarning,
        iconColor: '#3b82f6',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'blue' }).run(),
      },
      {
        title: 'Green callout',
        description: 'Success or tip',
        icon: MessageSquareWarning,
        iconColor: '#22c55e',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'green' }).run(),
      },
      {
        title: 'Grey callout',
        description: 'General note',
        icon: MessageSquareWarning,
        iconColor: '#9ca3af',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'grey' }).run(),
      },
      {
        title: 'Red callout',
        description: 'Warning or danger',
        icon: MessageSquareWarning,
        iconColor: '#ef4444',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'red' }).run(),
      },
      {
        title: 'Yellow callout',
        description: 'Caution or attention',
        icon: MessageSquareWarning,
        iconColor: '#eab308',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'yellow' }).run(),
      },
    ],
  },
  {
    title: 'Table',
    description: 'Insert a table',
    icon: Table,
    action: (editor) =>
      editor.chain().focus().insertTable({ rows: 2, cols: 2, withHeaderRow: true }).run(),
  },
  {
    title: 'Image',
    description: 'Upload an image',
    icon: Image,
    action: () => {
      // Handled specially in SlashMenu — triggers file picker
    },
  },
];
