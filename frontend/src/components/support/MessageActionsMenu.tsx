import {
  Comment01Icon,
  Copy01Icon,
  Delete01Icon,
  InformationCircleIcon,
  MoreVerticalIcon,
  PencilEdit02Icon,
  StickyNote01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu';
import { cn } from '@/lib/utils';

export interface MessageActions {
  canEdit: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onCopy: () => void;
  onReply: () => void;
  onDelete: () => void;
  onInfo: () => void;
  /** Optional — when provided, a "Save as shortcut" entry is shown. */
  onSaveAsShortcut?: () => void;
}

/**
 * Items component shape — same shadcn slot API for DropdownMenu and ContextMenu,
 * so we render the same set with whichever primitive is hosting them.
 */
interface ItemPrimitives {
  Item: typeof DropdownMenuItem | typeof ContextMenuItem;
  Separator: typeof DropdownMenuSeparator | typeof ContextMenuSeparator;
}

function MessageActionItems({
  Item,
  Separator,
  canEdit,
  canDelete,
  onEdit,
  onCopy,
  onReply,
  onDelete,
  onInfo,
  onSaveAsShortcut,
}: ItemPrimitives & MessageActions) {
  return (
    <>
      {canEdit && (
        <>
          <Item onSelect={onEdit}>
            <PencilEdit02Icon className="h-4 w-4" />
            Edit
          </Item>
          <Separator />
        </>
      )}
      <Item onSelect={onCopy}>
        <Copy01Icon className="h-4 w-4" />
        Copy
      </Item>
      <Item onSelect={onReply}>
        <Comment01Icon className="h-4 w-4" />
        Reply
      </Item>
      {onSaveAsShortcut && (
        <Item onSelect={onSaveAsShortcut}>
          <StickyNote01Icon className="h-4 w-4" />
          Save as shortcut
        </Item>
      )}
      {canDelete && (
        <Item variant="destructive" onSelect={onDelete}>
          <Delete01Icon className="h-4 w-4" />
          Delete
        </Item>
      )}
      <Separator />
      <Item onSelect={onInfo}>
        <InformationCircleIcon className="h-4 w-4" />
        Info
      </Item>
    </>
  );
}

interface MessageActionsMenuProps extends MessageActions {
  alignSide: 'left' | 'right';
}

export function MessageActionsMenu({ alignSide, ...actions }: MessageActionsMenuProps) {
  return (
    <div
      className={cn(
        'pointer-events-none absolute bottom-1 z-10 opacity-0 transition-opacity group-hover/message:opacity-100 group-focus-within/message:opacity-100',
        alignSide === 'left' ? '-left-7' : '-right-7',
      )}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="pointer-events-auto h-6 w-6 rounded-md text-foreground/70 hover:bg-muted hover:text-foreground"
            aria-label="Message actions"
          >
            <MoreVerticalIcon className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align={alignSide === 'left' ? 'end' : 'start'}
          side="bottom"
          className="min-w-44"
        >
          <MessageActionItems
            Item={DropdownMenuItem}
            Separator={DropdownMenuSeparator}
            {...actions}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

interface MessageActionsContextMenuProps extends MessageActions {
  children: React.ReactNode;
}

// Targets where the browser's native context menu is more useful than ours
// (open link in new tab, save image, paste into a field, etc.).
const NATIVE_MENU_SELECTOR =
  'a[href], img, video, audio, input, textarea, [contenteditable=""], [contenteditable="true"]';

function clickPointInSelection(e: React.MouseEvent, selection: Selection): boolean {
  // caretRangeFromPoint (WebKit/Blink) and caretPositionFromPoint (Gecko)
  // give us the exact text position under the click — this is what the
  // browser uses to decide whether the native menu shows selection actions.
  const doc = (e.target as Node)?.ownerDocument ?? document;
  type CaretAPI = Document & {
    caretRangeFromPoint?: (x: number, y: number) => Range | null;
    caretPositionFromPoint?: (
      x: number,
      y: number,
    ) => { offsetNode: Node; offset: number } | null;
  };
  const docApi = doc as CaretAPI;
  let caretNode: Node | null = null;
  let caretOffset = 0;
  if (typeof docApi.caretRangeFromPoint === 'function') {
    const r = docApi.caretRangeFromPoint(e.clientX, e.clientY);
    if (!r) return false;
    caretNode = r.startContainer;
    caretOffset = r.startOffset;
  } else if (typeof docApi.caretPositionFromPoint === 'function') {
    const p = docApi.caretPositionFromPoint(e.clientX, e.clientY);
    if (!p) return false;
    caretNode = p.offsetNode;
    caretOffset = p.offset;
  } else {
    return false;
  }
  for (let i = 0; i < selection.rangeCount; i++) {
    const range = selection.getRangeAt(i);
    if (
      range.comparePoint &&
      caretNode &&
      range.comparePoint(caretNode, caretOffset) === 0
    ) {
      return true;
    }
  }
  return false;
}

function shouldDeferToNative(e: React.MouseEvent): boolean {
  const target = e.target as Element | null;
  if (target && target.closest(NATIVE_MENU_SELECTOR)) return true;
  // Active text selection where the click point falls inside the selection —
  // let the user copy/search the selection via the browser menu.
  const selection = typeof window !== 'undefined' ? window.getSelection() : null;
  if (selection && !selection.isCollapsed && selection.toString().trim().length > 0) {
    if (clickPointInSelection(e, selection)) return true;
  }
  return false;
}

/**
 * Wraps a message bubble so right-click opens the same action menu as the
 * 3-dots trigger — except on links, images, inputs, or active text
 * selections, where the browser's native menu is more useful.
 */
export function MessageActionsContextMenu({ children, ...actions }: MessageActionsContextMenuProps) {
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        {/*
          Outer div is the Radix trigger element (asChild attaches its
          listeners here). The inner div sits below it in the DOM so its
          bubble-phase onContextMenu fires first; stopPropagation there
          prevents Radix from receiving the event, allowing the browser's
          native menu to render. display:contents keeps both wrappers out of
          the layout tree.
        */}
        <div style={{ display: 'contents' }}>
          <div
            style={{ display: 'contents' }}
            onContextMenu={(e) => {
              if (shouldDeferToNative(e)) e.stopPropagation();
            }}
          >
            {children}
          </div>
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent className="min-w-44">
        <MessageActionItems
          Item={ContextMenuItem}
          Separator={ContextMenuSeparator}
          {...actions}
        />
      </ContextMenuContent>
    </ContextMenu>
  );
}
