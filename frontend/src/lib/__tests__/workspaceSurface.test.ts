import { describe, expect, it } from 'vitest';
import { filterWorkspaceNav, workspaceHome, workspaceSurface } from '../workspaceSurface';

describe('deployment product surfaces', () => {
  it('opens support on a Community workspace and handles no access without redirect loops', () => {
    expect(workspaceHome('acme', ['support', 'docs', 'agents'])).toBe('/w/acme/support');
    expect(workspaceHome('acme', [])).toBeNull();
  });
  it('keeps agents separate from generic automation and PM', () => {
    expect(workspaceSurface('/w/acme/automation/agents')).toBe('agents');
    expect(workspaceSurface('/w/acme/automation/skills')).toBe('agents');
    expect(workspaceSurface('/w/acme/automation/flows')).toBe('automation');
    expect(workspaceSurface('/w/acme/pm/tasks')).toBe('pm');
    expect(filterWorkspaceNav([{ link: '/w/acme/automation/agents' }, { link: '/w/acme/automation/flows' }], ['agents'])).toEqual([{ link: '/w/acme/automation/agents' }]);
  });
});
