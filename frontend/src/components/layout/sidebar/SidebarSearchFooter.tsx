import { Search01Icon } from '@/lib/icons';
import { useSearchCommandStore } from '@/stores/searchCommandStore';

export function SidebarSearchFooter({ workspaceName }: { workspaceName?: string | null }) {
  const openSearch = useSearchCommandStore((state) => state.openSearch);
  const label = `Search ${workspaceName?.trim() || 'workspace'}`;

  return (
    <button
      type="button"
      aria-keyshortcuts="Meta+K Control+K"
      onClick={openSearch}
      className="flex min-h-11 w-full shrink-0 items-center gap-2 border-t border-quiet-divider-strong px-3 text-left text-[12.5px] text-quiet-text-tertiary transition-colors duration-150 hover:bg-quiet-hover hover:text-quiet-text-primary focus-visible:border-t-2 focus-visible:border-quiet-text-primary focus-visible:bg-quiet-hover focus-visible:text-quiet-text-primary focus-visible:outline-none motion-reduce:transition-none"
    >
      <Search01Icon className="h-[15px] w-[15px] shrink-0" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span aria-hidden="true" className="shrink-0 text-[11.5px] text-quiet-muted">⌘K</span>
    </button>
  );
}
