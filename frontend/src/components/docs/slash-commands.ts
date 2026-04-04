import type { Editor } from '@tiptap/core';
import {
  type IconComponent,
  SourceCodeIcon,
  Menu01Icon,
  PlayIcon,
  SmileIcon,
  Image01Icon,
  Heading02Icon,
  Heading03Icon,
  CheckListIcon,
  QuoteDownIcon,
  Message01Icon,
  MinusSignIcon,
  Table01Icon,
} from '@/lib/icons';

export interface SlashCommand {
  title: string;
  description: string;
  icon: IconComponent;
  iconColor?: string;
  action: (editor: Editor) => void;
  children?: SlashCommand[];
}

export const slashCommands: SlashCommand[] = [
  {
    title: 'Heading 2',
    description: 'Section heading',
    icon: Heading02Icon,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    title: 'Heading 3',
    description: 'Small section heading',
    icon: Heading03Icon,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    title: 'Bullet List',
    description: 'Unordered list',
    icon: Menu01Icon,
    action: (editor) => editor.chain().focus().toggleBulletList().run(),
  },
  {
    title: 'Numbered List',
    description: 'Ordered list',
    icon: CheckListIcon,
    action: (editor) => editor.chain().focus().toggleOrderedList().run(),
  },
  {
    title: 'Blockquote',
    description: 'Quote or excerpt',
    icon: QuoteDownIcon,
    action: (editor) => editor.chain().focus().toggleBlockquote().run(),
  },
  {
    title: 'Code Block',
    description: 'Fenced code block',
    icon: SourceCodeIcon,
    action: (editor) => editor.chain().focus().toggleCodeBlock().run(),
  },
  {
    title: 'Divider',
    description: 'Horizontal rule',
    icon: MinusSignIcon,
    action: (editor) => editor.chain().focus().setHorizontalRule().run(),
  },
  {
    title: 'Callout',
    description: 'Highlighted note or warning',
    icon: Message01Icon,
    action: () => {}, // parent — opens submenu
    children: [
      {
        title: 'Blue callout',
        description: 'Informational note',
        icon: Message01Icon,
        iconColor: '#3b82f6',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'blue' }).run(),
      },
      {
        title: 'Green callout',
        description: 'Success or tip',
        icon: Message01Icon,
        iconColor: '#22c55e',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'green' }).run(),
      },
      {
        title: 'Grey callout',
        description: 'General note',
        icon: Message01Icon,
        iconColor: '#9ca3af',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'grey' }).run(),
      },
      {
        title: 'Red callout',
        description: 'Warning or danger',
        icon: Message01Icon,
        iconColor: '#ef4444',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'red' }).run(),
      },
      {
        title: 'Yellow callout',
        description: 'Caution or attention',
        icon: Message01Icon,
        iconColor: '#eab308',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'yellow' }).run(),
      },
    ],
  },
  {
    title: 'Table',
    description: 'Insert a table',
    icon: Table01Icon,
    action: (editor) =>
      (editor.chain().focus() as any).insertTable({ rows: 2, cols: 2, withHeaderRow: true }).run(),
  },
  {
    title: 'Emoji',
    description: 'Insert an emoji',
    icon: SmileIcon,
    action: () => {
      // Handled specially in SlashMenu — opens emoji picker
    },
  },
  {
    title: 'HTML',
    description: 'Custom HTML block',
    icon: SourceCodeIcon,
    action: (editor) => editor.chain().focus().setHtmlBlock().run(),
  },
  {
    title: 'Video',
    description: 'Embed from YouTube, Vimeo, Loom, Wistia',
    icon: PlayIcon,
    action: () => {
      // Handled specially in SlashMenu — triggers video dialog
    },
  },
  {
    title: 'Image',
    description: 'Upload an image',
    icon: Image01Icon,
    action: () => {
      // Handled specially in SlashMenu — triggers file picker
    },
  },
];
