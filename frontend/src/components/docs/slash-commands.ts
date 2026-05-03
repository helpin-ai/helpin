import type { Editor } from '@tiptap/core';
import {
  Building03Icon,
  type IconComponent,
  DollarCircleIcon,
  FolderKanbanIcon,
  SourceCodeIcon,
  Menu01Icon,
  PlayIcon,
  SmileIcon,
  Image01Icon,
  File01Icon,
  AiMagicIcon,
  BookOpen01Icon,
  Heading02Icon,
  Heading03Icon,
  Heading04Icon,
  CheckListIcon,
  QuoteDownIcon,
  Message01Icon,
  MinusSignIcon,
  Table01Icon,
  LinkSquare01Icon,
  KanbanIcon,
  ArrowRight01Icon,
  LeftToRightListBulletIcon,
  UserIcon,
} from '@/lib/icons';
import { getDocsBlockDefinition } from './docsBlockRegistry';
import type { DocsEntityEmbedType } from './EntityEmbedExtension';

export interface SlashCommand {
  title: string;
  description: string;
  icon: IconComponent;
  iconColor?: string;
  action: (editor: Editor) => void;
  children?: SlashCommand[];
  entityType?: DocsEntityEmbedType;
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
    title: 'Heading 4',
    description: 'Sub-subsection heading',
    icon: Heading04Icon,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 4 }).run(),
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
    title: 'Task List',
    description: 'Checkboxes with owner and date',
    icon: CheckListIcon,
    action: (editor) =>
      editor.chain().focus().insertContent({
        type: 'taskList',
        content: [{ type: 'taskItem', attrs: { checked: false }, content: [{ type: 'paragraph' }] }],
      }).run(),
  },
  {
    title: 'Toggle',
    description: 'Collapsible section',
    icon: ArrowRight01Icon,
    action: (editor) => editor.chain().focus().setToggleSection({ title: 'Details', open: true }).run(),
  },
  {
    title: 'Table of Contents',
    description: 'Links to document headings',
    icon: LeftToRightListBulletIcon,
    action: (editor) => editor.chain().focus().setTableOfContents().run(),
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
    title: getDocsBlockDefinition('callout').label,
    description: getDocsBlockDefinition('callout').description,
    icon: Message01Icon,
    action: () => {}, // parent — opens submenu
    children: [
      {
        title: 'Info',
        description: 'Informational note',
        icon: Message01Icon,
        iconColor: '#3b82f6',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'info' }).run(),
      },
      {
        title: 'Warning',
        description: 'Caution or attention',
        icon: Message01Icon,
        iconColor: '#d97706',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'warning' }).run(),
      },
      {
        title: 'Tip',
        description: 'Helpful recommendation',
        icon: Message01Icon,
        iconColor: '#059669',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'tip' }).run(),
      },
      {
        title: 'Danger',
        description: 'Risk or destructive warning',
        icon: Message01Icon,
        iconColor: '#dc2626',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'danger' }).run(),
      },
      {
        title: 'Success',
        description: 'Positive outcome',
        icon: Message01Icon,
        iconColor: '#16a34a',
        action: (editor) => editor.chain().focus().setCallout({ variant: 'success' }).run(),
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
    title: getDocsBlockDefinition('aiSection').label,
    description: getDocsBlockDefinition('aiSection').description,
    icon: AiMagicIcon,
    action: (editor) => editor.chain().focus().setAISection({ status: 'draft' }).run(),
  },
  {
    title: getDocsBlockDefinition('citationBlock').label,
    description: getDocsBlockDefinition('citationBlock').description,
    icon: BookOpen01Icon,
    action: (editor) => editor.chain().focus().setCitationBlock({ title: 'Sources' }).run(),
  },
  {
    title: 'Task',
    description: 'Embed a PM task',
    icon: CheckListIcon,
    entityType: 'task',
    action: () => {
      // Handled specially in SlashMenu — opens task picker
    },
  },
  {
    title: 'Epic',
    description: 'Embed a PM epic',
    icon: FolderKanbanIcon,
    entityType: 'epic',
    action: () => {
      // Handled specially in SlashMenu — opens epic picker
    },
  },
  {
    title: 'Deal',
    description: 'Embed a CRM deal',
    icon: DollarCircleIcon,
    entityType: 'deal',
    action: () => {
      // Handled specially in SlashMenu — opens deal picker
    },
  },
  {
    title: 'Contact',
    description: 'Embed a CRM contact',
    icon: UserIcon,
    entityType: 'contact',
    action: () => {
      // Handled specially in SlashMenu — opens contact picker
    },
  },
  {
    title: 'Company',
    description: 'Embed a CRM company',
    icon: Building03Icon,
    entityType: 'company',
    action: () => {
      // Handled specially in SlashMenu — opens company picker
    },
  },
  {
    title: 'Conversation',
    description: 'Embed a support conversation',
    icon: Message01Icon,
    entityType: 'support_conversation',
    action: () => {
      // Handled specially in SlashMenu — opens conversation picker
    },
  },
  {
    title: getDocsBlockDefinition('entityEmbed').label,
    description: 'Search across tasks, epics, CRM records, and conversations',
    icon: LinkSquare01Icon,
    action: () => {
      // Handled specially in SlashMenu — opens generic entity picker
    },
  },
  {
    title: getDocsBlockDefinition('savedViewEmbed').label,
    description: getDocsBlockDefinition('savedViewEmbed').description,
    icon: KanbanIcon,
    action: (editor) => editor.chain().focus().setSavedViewEmbed({ module: 'pm' }).run(),
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
    title: getDocsBlockDefinition('excalidraw').label,
    description: getDocsBlockDefinition('excalidraw').description,
    icon: SourceCodeIcon,
    action: (editor) => editor.chain().focus().setExcalidraw().run(),
  },
  {
    title: 'Video',
    description: getDocsBlockDefinition('videoEmbed').description,
    icon: PlayIcon,
    action: () => {
      // Handled specially in SlashMenu — triggers video dialog
    },
  },
  {
    title: 'Embed',
    description: 'Unfurl a workspace link',
    icon: LinkSquare01Icon,
    action: () => {
      // Handled specially in SlashMenu — triggers embed dialog
    },
  },
  {
    title: getDocsBlockDefinition('resizableImage').label,
    description: getDocsBlockDefinition('resizableImage').description,
    icon: Image01Icon,
    action: () => {
      // Handled specially in SlashMenu — triggers file picker
    },
  },
  {
    title: 'File',
    description: 'Attach a downloadable file',
    icon: File01Icon,
    action: () => {
      // Handled specially in SlashMenu — triggers file picker
    },
  },
];
