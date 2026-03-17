import { describe, expect, it } from 'vitest';

import { WORKSPACE_TEAM_SUGGESTIONS } from '../teamPresets';

describe('WORKSPACE_TEAM_SUGGESTIONS', () => {
  it('marks only Engineering and Product as engineering teams', () => {
    const teamTypes = Object.fromEntries(
      WORKSPACE_TEAM_SUGGESTIONS.map((team) => [team.name, team.teamType]),
    );

    expect(teamTypes).toMatchObject({
      Engineering: 'engineering',
      Product: 'engineering',
      Design: 'custom',
      Support: 'custom',
      Marketing: 'custom',
    });
  });
});
