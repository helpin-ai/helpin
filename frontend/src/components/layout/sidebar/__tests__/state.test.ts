// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from 'vitest';
import { getLastCRMPath, normalizeCRMSectionPath, saveLastCRMPath } from '../state';

describe('CRM sidebar route memory', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults a new workspace to Overview', () => {
    expect(getLastCRMPath('workspace-1', 'acme')).toBe('/w/acme/crm/overview');
  });

  it('remembers the last CRM section for each workspace', () => {
    saveLastCRMPath('workspace-1', 'acme', '/w/acme/crm/contacts');
    saveLastCRMPath('workspace-2', 'other', '/w/other/crm/deals');

    expect(getLastCRMPath('workspace-1', 'acme')).toBe('/w/acme/crm/contacts');
    expect(getLastCRMPath('workspace-2', 'other')).toBe('/w/other/crm/deals');
  });

  it('returns to a section rather than reopening a record detail', () => {
    saveLastCRMPath('workspace-1', 'acme', '/w/acme/crm/contacts/contact-1');
    expect(getLastCRMPath('workspace-1', 'acme')).toBe('/w/acme/crm/contacts');
  });

  it('rejects paths outside known CRM sections', () => {
    expect(normalizeCRMSectionPath('acme', '/w/acme/settings/general')).toBe('/w/acme/crm/overview');
    expect(normalizeCRMSectionPath('acme', '/w/other/crm/contacts')).toBe('/w/acme/crm/overview');
  });
});
