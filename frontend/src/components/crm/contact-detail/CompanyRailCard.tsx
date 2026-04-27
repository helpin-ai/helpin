import { Building03Icon, MoreVerticalIcon } from '@/lib/icons';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { getInitials } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

interface CompanyRailCardProps {
  name: string;
  displayId?: string;
  onOpen: () => void;
  onRemove: () => void;
  onChangePrimary?: () => void;
}

export function CompanyRailCard({
  name,
  displayId,
  onOpen,
  onRemove,
  onChangePrimary,
}: CompanyRailCardProps) {
  return (
    <div className="group flex items-center gap-2.5 rounded-lg border border-border/60 bg-background px-2.5 py-2 transition-colors hover:border-border">
      <Avatar className="size-7 rounded-md bg-muted">
        <AvatarFallback className="rounded-md bg-muted text-[10px] font-semibold">
          {name ? getInitials(name) : <Building03Icon className="h-3.5 w-3.5" />}
        </AvatarFallback>
      </Avatar>
      <button
        type="button"
        onClick={onOpen}
        className="min-w-0 flex-1 text-left"
      >
        <div className="truncate text-[13px] font-medium text-foreground">{name || 'Untitled company'}</div>
        {displayId && (
          <div className="truncate font-mono text-[10.5px] text-muted-foreground">{displayId}</div>
        )}
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="rounded-md p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100 focus:opacity-100"
            aria-label="Company actions"
          >
            <MoreVerticalIcon className="h-3.5 w-3.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={onOpen}>Open company</DropdownMenuItem>
          {onChangePrimary && (
            <DropdownMenuItem onClick={onChangePrimary}>Change primary</DropdownMenuItem>
          )}
          <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={onRemove}>
            Unlink
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
