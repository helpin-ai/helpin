import type { SearchResult } from '@/lib/services/searchService';

export function normalizeCommandSearchText(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
    .replace(/\s+/g, ' ');
}

export function buildSearchCommandValue(type: string, item: Pick<SearchResult, 'id' | 'name'>): string {
  return [
    type,
    item.id,
    item.name,
    normalizeCommandSearchText(item.name),
  ]
    .filter(Boolean)
    .join(' ');
}

export function buildTaskCommandValue(
  item: Pick<SearchResult, 'id' | 'task_key' | 'display_id' | 'name'>,
): string {
  return [
    'task',
    item.id,
    item.task_key,
    item.display_id != null ? String(item.display_id) : '',
    item.name,
    normalizeCommandSearchText(item.name),
  ]
    .filter(Boolean)
    .join(' ');
}
