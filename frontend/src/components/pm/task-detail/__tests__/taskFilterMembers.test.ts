import { describe, expect, it } from 'vitest';

import type { AssignableMember, TeamUserMembership } from '@/lib/types';

import { filterAssignableMembersForTeam } from '../taskFilterMembers';

describe('filterAssignableMembersForTeam', () => {
  const members: AssignableMember[] = [
    {
      id: 'member-1',
      user_id: 'user-1',
      role: 'member',
      email: 'one@example.com',
      display_name: 'One',
      status: 'active',
    },
    {
      id: 'member-2',
      user_id: 'user-2',
      role: 'member',
      email: 'two@example.com',
      display_name: 'Two',
      status: 'active',
    },
    {
      id: 'member-3',
      user_id: 'user-3',
      role: 'member',
      email: 'three@example.com',
      display_name: 'Three',
      status: 'inactive',
    },
    {
      id: 'member-4',
      role: 'member',
      email: 'invite@example.com',
      display_name: 'Invite',
      status: 'active',
    },
  ];

  const userMemberships: TeamUserMembership[] = [
    {
      id: 'tum-1',
      team_id: 'team-a',
      user_id: 'user-1',
      role: 'member',
      created_at: '',
      updated_at: '',
    },
    {
      id: 'tum-2',
      team_id: 'team-b',
      user_id: 'user-2',
      role: 'member',
      created_at: '',
      updated_at: '',
    },
  ];

  it('returns all active assignable members when no team is selected', () => {
    expect(filterAssignableMembersForTeam(members, null, userMemberships).map((member) => member.id)).toEqual([
      'member-1',
      'member-2',
      'member-4',
    ]);
  });

  it('returns only active members assigned to the selected team', () => {
    expect(filterAssignableMembersForTeam(members, 'team-a', userMemberships).map((member) => member.id)).toEqual([
      'member-1',
    ]);
  });
});
