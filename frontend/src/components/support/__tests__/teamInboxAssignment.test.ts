import { describe, expect, it } from 'vitest';
import { filterInboxAssignmentMembers, resolveInboxAssignmentSelection } from '../teamInboxAssignment';

const members = [
  { id: 'owner', user_id: 'u-owner', role: 'owner', status: 'active' },
  { id: 'team', user_id: 'u-team', role: 'member', status: 'active' },
  { id: 'extra', user_id: 'u-extra', role: 'member', status: 'active' },
  { id: 'outsider', user_id: 'u-outsider', role: 'member', status: 'active' },
  { id: 'viewer', user_id: 'u-viewer', role: 'viewer', status: 'active' },
  { id: 'revoked', user_id: 'u-revoked', role: 'member', status: 'revoked' },
  { id: 'pending', role: 'member', status: 'pending' },
];
const memberships = [{ team_id: 'support', user_id: 'u-team' }];

describe('inbox assignment', () => {
  it('only offers active people with inbox access who can handle conversations', () => {
    expect(filterInboxAssignmentMembers(members, 'support', ['extra', 'viewer', 'revoked', 'pending'], memberships).map(m => m.id))
      .toEqual(['owner', 'team', 'extra']);
  });

  it('does not include the previous team after access changes', () => {
    expect(filterInboxAssignmentMembers(members, 'none', ['extra'], memberships).map(m => m.id))
      .toEqual(['owner', 'extra']);
  });

  it('never interprets an empty explicit pool as everyone', () => {
    expect(resolveInboxAssignmentSelection([], members, [])).toEqual([]);
  });

  it('removes selections that lost access', () => {
    expect(resolveInboxAssignmentSelection(['extra', 'removed'], members, [])).toEqual(['extra']);
  });

  it('preserves only the existing inbox members for a legacy round-robin pool', () => {
    expect(resolveInboxAssignmentSelection(null, members, ['team', 'extra'])).toEqual(['team', 'extra']);
  });
});
