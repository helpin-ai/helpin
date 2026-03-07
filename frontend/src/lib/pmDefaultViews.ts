import type { PMView } from './pmTypes';

export function getDefaultViews(currentMemberId: string): PMView[] {
  const now = new Date().toISOString();
  const oneWeekAgo = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString();

  return [
    {
      id: '__default_everything__',
      workspace_id: '',
      name: 'Everything',
      filters: {},
      is_shared: false,
      is_pinned: true,
      position: -4,
      created_by: '',
      created_at: now,
      updated_at: now,
    },
    {
      id: '__default_owned_by_me__',
      workspace_id: '',
      name: 'Owned by me',
      filters: { owner_member_id: currentMemberId },
      is_shared: false,
      is_pinned: true,
      position: -3,
      created_by: '',
      created_at: now,
      updated_at: now,
    },
    {
      id: '__default_requested_by_me__',
      workspace_id: '',
      name: 'Requested by me',
      filters: { requester_member_id: currentMemberId },
      is_shared: false,
      is_pinned: true,
      position: -2,
      created_by: '',
      created_at: now,
      updated_at: now,
    },
    {
      id: '__default_updated_last_week__',
      workspace_id: '',
      name: 'Updated last week',
      filters: { updated_after: oneWeekAgo },
      is_shared: false,
      is_pinned: true,
      position: -1,
      created_by: '',
      created_at: now,
      updated_at: now,
    },
  ];
}

export function isDefaultView(viewId: string): boolean {
  return viewId.startsWith('__default_');
}
