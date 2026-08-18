export type DockDateGroup = 'today' | 'yesterday' | 'previous_7_days' | 'older';

export const DOCK_DATE_GROUPS: Array<{ key: DockDateGroup; label: string }> = [
  { key: 'today', label: 'Today' },
  { key: 'yesterday', label: 'Yesterday' },
  { key: 'previous_7_days', label: 'Previous 7 days' },
  { key: 'older', label: 'Older' },
];

export function dockDateGroup(timestamp: string, now = new Date()): DockDateGroup {
  const value = new Date(timestamp);
  if (!Number.isFinite(value.getTime())) return 'older';
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const day = new Date(value.getFullYear(), value.getMonth(), value.getDate());
  const daysAgo = Math.round((today.getTime() - day.getTime()) / 86_400_000);
  if (daysAgo <= 0) return 'today';
  if (daysAgo === 1) return 'yesterday';
  if (daysAgo <= 7) return 'previous_7_days';
  return 'older';
}
