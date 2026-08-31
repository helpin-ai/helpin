import { useEffect } from 'react';

import { SearchCommandPalette } from '@/components/search/SearchCommandPalette';
import { useSearchCommandStore } from '@/stores/searchCommandStore';

export function WorkspaceCommandSearch() {
  const open = useSearchCommandStore((state) => state.open);
  const setOpen = useSearchCommandStore((state) => state.setOpen);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return;
      if (event.key.toLowerCase() !== 'k' || (!event.metaKey && !event.ctrlKey)) return;

      event.preventDefault();
      setOpen(true);
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [setOpen]);

  return <SearchCommandPalette open={open} onOpenChange={setOpen} />;
}
