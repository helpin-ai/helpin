import {
  Comment01Icon,
  Copy01Icon,
  Delete01Icon,
  InformationCircleIcon,
  MoreVerticalIcon,
  PencilEdit02Icon,
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
          className="min-w-32"
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

/**
 * Wraps a message bubble so right-click opens the same action menu as the
 * 3-dots trigger.
 */
export function MessageActionsContextMenu({ children, ...actions }: MessageActionsContextMenuProps) {
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="min-w-32">
        <MessageActionItems
          Item={ContextMenuItem}
          Separator={ContextMenuSeparator}
          {...actions}
        />
      </ContextMenuContent>
    </ContextMenu>
  );
}
