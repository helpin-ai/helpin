import { describe, expect, it } from 'vitest';
import {
  buildSupportInboxSearch,
  navFilterFromView,
  viewFromNavFilter,
} from '../supportInboxRouting';

describe('supportInboxRouting', () => {
  it('maps canonical support views to nav filters', () => {
    expect(navFilterFromView(undefined)).toBe('inbox');
    expect(navFilterFromView('inbox')).toBe('inbox');
    expect(navFilterFromView('mine')).toBe('mine');
    expect(navFilterFromView('waiting')).toBe('waiting');
    expect(navFilterFromView('resolved')).toBe('resolved');
    expect(navFilterFromView('spam')).toBe('spam');
    expect(navFilterFromView('ai-handling')).toBe('ai_active');
    expect(navFilterFromView('ai-resolved')).toBe('resolved_by_ai');
  });

  it('preserves old support view URLs as aliases', () => {
    expect(navFilterFromView('all')).toBe('inbox');
    expect(navFilterFromView('assigned')).toBe('mine');
    expect(navFilterFromView('my')).toBe('mine');
    expect(navFilterFromView('my_inbox')).toBe('mine');
    expect(navFilterFromView('mentions')).toBe('mine');
    expect(navFilterFromView('unassigned')).toBe('inbox');
    expect(navFilterFromView('ai-active')).toBe('ai_active');
    expect(navFilterFromView('resolved_by_ai')).toBe('resolved_by_ai');
  });

  it('builds compact global inbox URLs and explicit team inbox URLs', () => {
    expect(viewFromNavFilter('inbox')).toBe('inbox');
    expect(viewFromNavFilter('mine')).toBe('mine');

    expect(buildSupportInboxSearch({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      statusFilter: 'all',
      searchQuery: '',
    })).toEqual({});

    expect(buildSupportInboxSearch({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      statusFilter: 'all',
      searchQuery: '',
    })).toEqual({
      view: 'team',
      team_inbox: 'mailbox-billing',
    });
  });

  it('builds custom view URLs by selected view type and id', () => {
    expect(navFilterFromView('team')).toBe('inbox');
    expect(navFilterFromView('custom')).toBe('inbox');

    expect(buildSupportInboxSearch({
      navFilter: 'waiting',
      selectedMailboxId: 'mailbox-billing',
      statusFilter: 'all',
      searchQuery: '',
      activeCustomViewId: 'view-billing',
      listFilters: {
        states: ['waiting_on_customer'],
        assignment: ['me'],
        mailboxIds: ['mailbox-billing'],
        tagIds: ['tag-billing'],
        aiStates: ['handoff'],
        sort: 'oldest',
      },
      includeFilterParams: false,
    })).toEqual({
      view: 'custom',
      custom_view: 'view-billing',
    });
  });

  it('can keep saved team view URLs clean when filters are represented by the view', () => {
    expect(buildSupportInboxSearch({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      statusFilter: 'all',
      searchQuery: 'refund',
      listFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: [],
        mailboxIds: ['mailbox-billing'],
        tagIds: ['tag-billing'],
        aiStates: ['handoff'],
        sort: 'oldest',
      },
      includeFilterParams: false,
    })).toEqual({
      view: 'team',
      team_inbox: 'mailbox-billing',
    });
  });

  it('stores explicit state selections only when they differ from the sidebar default', () => {
    expect(buildSupportInboxSearch({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      statusFilter: 'all',
      searchQuery: '',
      listFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me', 'mentioned_me', 'opened_by_me'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })).toEqual({ view: 'mine' });

    expect(buildSupportInboxSearch({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      statusFilter: 'all',
      searchQuery: '',
      listFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })).toEqual({});
  });

  it('stores Assignment filters only when they differ from the sidebar default', () => {
    expect(buildSupportInboxSearch({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      statusFilter: 'all',
      searchQuery: '',
      listFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: ['me'],
        mailboxIds: [],
        tagIds: [],
        aiStates: [],
        sort: 'newest',
      },
    })).toEqual({
      view: 'mine',
      assigned_to: 'me',
    });
  });

  it('stores tag and AI state filters in the URL', () => {
    expect(buildSupportInboxSearch({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      statusFilter: 'all',
      searchQuery: '',
      listFilters: {
        states: ['open', 'waiting_on_customer'],
        assignment: [],
        mailboxIds: [],
        tagIds: ['tag-billing'],
        aiStates: ['handoff'],
        sort: 'newest',
      },
    })).toEqual({
      tag_ids: 'tag-billing',
      ai: 'handoff',
    });
  });
});
