import { describe, expect, it } from 'vitest';

import {
  filterMembersOutsideLinkedTeam,
  filterSupportAccessibleMembers,
  filterSupportAccessibleTeams,
  splitMailboxMembersBySelection,
} from '../teamInboxDialogMembers';

describe('team inbox dialog members', () => {
  it('puts selected members first and removes them from the available list', () => {
    const members = [
      { id: 'wm-1', display_name: 'Ada Lovelace', email: 'ada@example.com' },
      { id: 'wm-2', display_name: 'Grace Hopper', email: 'grace@example.com' },
      { id: 'wm-3', display_name: 'Katherine Johnson', email: 'katherine@example.com' },
    ];

    const result = splitMailboxMembersBySelection(members, ['wm-3', 'wm-1']);

    expect(result.selected.map((member) => member.id)).toEqual(['wm-3', 'wm-1']);
    expect(result.available.map((member) => member.id)).toEqual(['wm-2']);
  });

  it('shows only teams that have support module access', () => {
    const teams = [
      { id: 'team-support', name: 'Support' },
      { id: 'team-sales', name: 'Sales' },
    ];
    const grants = [
      { module: 'support', subject_type: 'team', subject_id: 'team-support' },
      { module: 'crm', subject_type: 'team', subject_id: 'team-sales' },
    ];

    expect(filterSupportAccessibleTeams(teams, grants).map((team) => team.id)).toEqual(['team-support']);
  });

  it('shows members with direct, inherited, or admin support access', () => {
    const members = [
      { id: 'wm-owner', user_id: 'u-owner', role: 'owner' },
      { id: 'wm-direct', user_id: 'u-direct', role: 'member' },
      { id: 'wm-team', user_id: 'u-team', role: 'member' },
      { id: 'wm-hidden', user_id: 'u-hidden', role: 'member' },
    ];
    const grants = [
      { module: 'support', subject_type: 'workspace_member', subject_id: 'wm-direct' },
      { module: 'support', subject_type: 'team', subject_id: 'team-support' },
    ];
    const teamMemberships = [
      { team_id: 'team-support', user_id: 'u-team' },
      { team_id: 'team-sales', user_id: 'u-hidden' },
    ];

    expect(
      filterSupportAccessibleMembers(members, grants, teamMemberships).map((member) => member.id),
    ).toEqual(['wm-owner', 'wm-direct', 'wm-team']);
  });

  it('removes linked team members from the additional member list', () => {
    const members = [
      { id: 'wm-team', user_id: 'u-team', role: 'member' },
      { id: 'wm-extra', user_id: 'u-extra', role: 'member' },
      { id: 'wm-pending-profile', role: 'member' },
    ];
    const teamMemberships = [
      { team_id: 'team-support', user_id: 'u-team' },
      { team_id: 'team-sales', user_id: 'u-extra' },
    ];

    expect(
      filterMembersOutsideLinkedTeam(members, 'team-support', teamMemberships).map((member) => member.id),
    ).toEqual(['wm-extra', 'wm-pending-profile']);
  });
});
