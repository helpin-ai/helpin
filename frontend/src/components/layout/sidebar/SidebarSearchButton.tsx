import { Search01Icon } from '@/lib/icons';
import { useSearchCommandStore } from '@/stores/searchCommandStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { isMacPlatform } from '../workspaceSwitcherShortcuts';

export function SidebarSearchButton() {
  const openSearch = useSearchCommandStore((state) => state.openSearch);
  const shortcut = isMacPlatform() ? '⌘K' : 'Ctrl+K';

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label="Search your workspace"
          aria-keyshortcuts="Meta+K Control+K"
          onClick={openSearch}
          className="flex size-11 shrink-0 cursor-pointer md:size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <Search01Icon className="h-4 w-4" aria-hidden="true" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="right">
        <span className="flex items-center gap-2">
          Search your workspace
          <kbd className="text-xs opacity-70">{shortcut}</kbd>
        </span>
      </TooltipContent>
    </Tooltip>
  );
}
