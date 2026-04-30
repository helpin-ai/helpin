import {
  Comment01Icon,
  Copy01Icon,
  Delete01Icon,
  InformationCircleIcon,
  MoreHorizontalIcon,
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
import { cn } from '@/lib/utils';

interface MessageActionsMenuProps {
  alignSide: 'left' | 'right';
  canEdit: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onCopy: () => void;
  onReply: () => void;
  onDelete: () => void;
  onInfo: () => void;
}

export function MessageActionsMenu({
  alignSide,
  canEdit,
  canDelete,
  onEdit,
  onCopy,
  onReply,
  onDelete,
  onInfo,
}: MessageActionsMenuProps) {
  return (
    <div
      className={cn(
        'pointer-events-none absolute top-1/2 z-10 -translate-y-1/2 opacity-0 transition-opacity group-hover/message:opacity-100 group-focus-within/message:opacity-100',
        alignSide === 'left' ? '-left-9' : '-right-9',
      )}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            className="pointer-events-auto h-7 w-7 rounded-full bg-background/95 shadow-sm"
            aria-label="Message actions"
          >
            <MoreHorizontalIcon className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align={alignSide === 'left' ? 'end' : 'start'} side="bottom">
          {canEdit && (
            <>
              <DropdownMenuItem onSelect={onEdit}>
                <PencilEdit02Icon className="h-4 w-4" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}
          <DropdownMenuItem onSelect={onCopy}>
            <Copy01Icon className="h-4 w-4" />
            Copy
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={onReply}>
            <Comment01Icon className="h-4 w-4" />
            Reply
          </DropdownMenuItem>
          {canDelete && (
            <DropdownMenuItem variant="destructive" onSelect={onDelete}>
              <Delete01Icon className="h-4 w-4" />
              Delete
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={onInfo}>
            <InformationCircleIcon className="h-4 w-4" />
            Info
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
