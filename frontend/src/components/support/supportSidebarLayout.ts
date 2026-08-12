import type { DetailSidebarMode } from '@/stores/supportInboxStore';

export function supportSidebarWidthClass(mode: DetailSidebarMode, detailsCollapsed: boolean): string {
  if (mode === 'agents') return 'w-[420px]';
  return detailsCollapsed ? 'w-10' : 'w-[300px]';
}
