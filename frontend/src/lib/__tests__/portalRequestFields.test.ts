import { describe, expect, it } from 'vitest';

import { portalFieldsForSubmit } from '../portalRequestFields';
import type { CustomerPortalRequestType } from '../services/customerPortalService';

const bug: CustomerPortalRequestType = {
  id: 'bug',
  name: 'Bug report',
  description: '',
  fields: [
    { key: 'severity', label: 'Severity', field_type: 'select', options: [{ value: 'high', label: 'High' }], required: true, help_text: '' },
    { key: 'seats', label: 'Seats', field_type: 'number', options: [], required: false, help_text: '' },
    { key: 'paid', label: 'Paid plan', field_type: 'boolean', options: [], required: false, help_text: '' },
    { key: 'areas', label: 'Areas', field_type: 'multi_select', options: [{ value: 'api', label: 'API' }], required: false, help_text: '' },
  ],
};

describe('portalFieldsForSubmit', () => {
  it('converts values and leaves out empty ones', () => {
    expect(portalFieldsForSubmit(bug, { severity: 'high', seats: ' 12 ', paid: 'false', areas: ['api'], other: 'x' })).toEqual({
      fields: { severity: 'high', seats: 12, paid: false, areas: ['api'] },
      error: null,
    });
    expect(portalFieldsForSubmit(bug, { severity: 'high', seats: '', areas: [] }).fields).toEqual({ severity: 'high' });
  });

  it('reports a missing required field and a bad number', () => {
    expect(portalFieldsForSubmit(bug, {}).error).toBe('Severity is required.');
    expect(portalFieldsForSubmit(bug, { severity: 'high', seats: 'many' }).error).toBe('Seats must be a number.');
  });

  it('has nothing to send without a type', () => {
    expect(portalFieldsForSubmit(undefined, { severity: 'high' })).toEqual({ fields: {}, error: null });
  });
});
