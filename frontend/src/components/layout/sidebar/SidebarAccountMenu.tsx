import { Logout01Icon, Setting07Icon, UserIcon, UserGroupIcon } from '@/lib/icons';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

type SupportPresence = {
  status: 'online' | 'away' | 'offline';
  manual_status?: 'online' | 'away' | 'offline' | 'auto' | null;
};

type SidebarAccountMenuProps = {
  user?: {
    full_name?: string | null;
    email?: string | null;
    avatar_url?: string | null;
  } | null;
  initials: string;
  presence: SupportPresence | null;
  selectedPresenceMode: 'online' | 'away' | 'offline' | 'auto';
  onPresenceChange: (value: 'online' | 'away' | 'offline' | 'auto') => void;
  onProfile: () => void;
  onSettings: () => void;
  onWorkspaces: () => void;
  onSignOut: () => void;
};

function getPresenceColor(status: SupportPresence['status']) {
  if (status === 'online') return 'bg-emerald-500';
  if (status === 'away') return 'bg-amber-500';
  return 'bg-slate-300 dark:bg-slate-600';
}

export function SidebarAccountMenu({
  user,
  initials,
  presence,
  selectedPresenceMode,
  onPresenceChange,
  onProfile,
  onSettings,
  onWorkspaces,
  onSignOut,
}: SidebarAccountMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="relative rounded-full transition-opacity hover:opacity-80"
          aria-label="Account menu"
        >
          <Avatar className="size-8">
            <AvatarImage src={user?.avatar_url ?? undefined} alt={user?.full_name || user?.email || 'Account'} />
            <AvatarFallback className="text-[11px]">
              {initials}
            </AvatarFallback>
          </Avatar>
          {presence && (
            <span className={`absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-background ${getPresenceColor(presence.status)}`} />
          )}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="end" className="w-56">
        <DropdownMenuLabel className="truncate">
          {user?.full_name || user?.email || 'Account'}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        {presence && (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <span className="flex items-center gap-2">
                <span className={`h-2 w-2 rounded-full ${getPresenceColor(presence.status)}`} />
                <span>Support status</span>
              </span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="w-44">
              <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
                Set your status
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuRadioGroup
                value={selectedPresenceMode}
                onValueChange={(value) => onPresenceChange(value as 'online' | 'away' | 'offline' | 'auto')}
              >
                <DropdownMenuRadioItem value="auto">Automatic</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="online">Online</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="away">Away</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="offline">Offline</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}
        {presence && <DropdownMenuSeparator />}
        <DropdownMenuItem onClick={onProfile}>
          <UserIcon className="h-4 w-4" />
          <span>Profile</span>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onSettings}>
          <Setting07Icon className="h-4 w-4" />
          <span>Settings</span>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onWorkspaces}>
          <UserGroupIcon className="h-4 w-4" />
          <span>All Workspaces</span>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onSignOut} variant="destructive">
          <Logout01Icon className="h-4 w-4" />
          <span>Sign out</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
