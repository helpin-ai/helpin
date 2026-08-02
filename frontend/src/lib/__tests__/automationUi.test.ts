import { describe, expect, it } from 'vitest';
import {
  buildAutomationToolConnectionsPath,
  buildAutomationToolsPath,
} from '../automationUi';

describe('automation tool paths', () => {
  it('keeps the tool catalog at the existing URL', () => {
    expect(buildAutomationToolsPath('acme')).toBe('/w/acme/automation/tools');
  });

  it('builds a workspace-scoped external connections URL', () => {
    expect(buildAutomationToolConnectionsPath('acme')).toBe('/w/acme/automation/tools/connections');
  });
});
