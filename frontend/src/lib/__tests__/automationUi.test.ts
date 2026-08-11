import { describe, expect, it } from 'vitest';
import { buildAutomationToolsPath } from '../automationUi';

describe('automation tool paths', () => {
  it('keeps the tool catalog at the existing URL', () => {
    expect(buildAutomationToolsPath('acme')).toBe('/w/acme/automation/tools');
  });
});
