import type { SearchResult } from '@/lib/services/searchService';

export function buildTaskCommandValue(
  item: Pick<SearchResult, 'id' | 'task_key' | 'display_id' | 'name'>,
): string {
  return [
    'task',
    item.id,
    item.task_key,
    item.display_id != null ? String(item.display_id) : '',
    item.name,
  ]
    .filter(Boolean)
    .join(' ');
}
