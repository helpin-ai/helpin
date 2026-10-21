// @vitest-environment jsdom

import { afterEach, describe, expect, it } from 'vitest';

import {
  buildCompanyFilterDefinitions,
  companyFilterGroupToValues,
  companyFilterValuesToGroup,
  customPropertyColumnId,
  customPropertyFilterField,
  effectiveCompanyColumnVisibility,
  readCompanyColumnVisibility,
  removeCompanyFilter,
  toggleCompanyFilterValue,
  writeCompanyColumnVisibility,
} from '../crmCompanyFilters';
import { parseQueryFilterGroup, serializeQueryFilterGroup } from '../queryBuilder';
import type { CRMCompanyFilterOptions } from '../crmTypes';
import type { AssignableMember } from '../types';

describe('company filter values', () => {
  it('turns selections into one is_any_of rule per field and back', () => {
    const values = {
      [customPropertyFilterField('fit')]: ['yes', 'maybe'],
      industry: ['Software'],
      owner_member_id: [],
    };
    const group = companyFilterValuesToGroup(values);

    expect(group).toEqual({
      logic: 'and',
      rules: [
        { field: 'custom_properties.fit', operator: 'is_any_of', values: ['yes', 'maybe'] },
        { field: 'industry', operator: 'is_any_of', values: ['Software'] },
      ],
    });
    expect(companyFilterGroupToValues(group)).toEqual({
      'custom_properties.fit': ['yes', 'maybe'],
      industry: ['Software'],
    });
  });

  it('survives URL serialization, keeping the value list', () => {
    const group = companyFilterValuesToGroup({ 'custom_properties.support_tools': ['zendesk'] });
    const parsed = parseQueryFilterGroup(serializeQueryFilterGroup(group));
    expect(companyFilterGroupToValues(parsed)).toEqual({ 'custom_properties.support_tools': ['zendesk'] });
  });

  it('returns no group without selections and ignores other operators when reading', () => {
    expect(companyFilterValuesToGroup({ industry: [] })).toBeUndefined();
    expect(companyFilterGroupToValues({ logic: 'and', rules: [{ field: 'name', operator: 'contains', value: 'acme' }] })).toEqual({});
  });

  it('toggles and removes values without mutating the input', () => {
    const start = { industry: ['Software'] };
    expect(toggleCompanyFilterValue(start, 'industry', 'Retail')).toEqual({ industry: ['Software', 'Retail'] });
    expect(toggleCompanyFilterValue(start, 'industry', 'Software')).toEqual({ industry: [] });
    expect(removeCompanyFilter(start, 'industry')).toEqual({});
    expect(start).toEqual({ industry: ['Software'] });
  });
});

describe('buildCompanyFilterDefinitions', () => {
  const options: CRMCompanyFilterOptions = {
    industries: [{ value: 'Software', count: 3 }],
    custom_properties: [
      { key: 'is_b2b', count: 2, value_type: 'boolean', values: [{ value: 'true', count: 2 }, { value: 'false', count: 1 }], has_more_values: false },
      { key: 'support_tools', count: 2, value_type: 'list', values: [{ value: 'zendesk', count: 2 }], has_more_values: false },
    ],
    scanned_companies: 3,
    truncated: false,
  };
  const members = [{ id: 'm1', display_name: 'Ada Lovelace', email: 'ada@example.com' }] as AssignableMember[];

  it('puts Owner first, then Industry, then custom properties with readable labels', () => {
    const definitions = buildCompanyFilterDefinitions(options, members);
    expect(definitions.map((definition) => definition.label)).toEqual(['Owner', 'Industry', 'Is b2b', 'Support tools']);
    expect(definitions[0].options).toEqual([{ value: 'm1', label: 'Ada Lovelace' }]);
    expect(definitions[2].key).toBe('custom_properties.is_b2b');
    expect(definitions[2].options.map((option) => option.label)).toEqual(['Yes', 'No']);
    expect(definitions[3].options).toEqual([{ value: 'zendesk', label: 'zendesk' }]);
  });

  it('still offers Owner before filter options load', () => {
    expect(buildCompanyFilterDefinitions(undefined, members).map((definition) => definition.key)).toEqual(['owner_member_id', 'industry']);
  });
});

describe('company column visibility', () => {
  afterEach(() => window.localStorage.clear());

  it('hides custom property columns until the user shows them', () => {
    const fit = customPropertyColumnId('fit');
    const batch = customPropertyColumnId('batch');
    expect(effectiveCompanyColumnVisibility({ [fit]: true, industryName: false }, [fit, batch])).toEqual({
      [fit]: true,
      [batch]: false,
      industryName: false,
    });
  });

  it('saves visibility per workspace and ignores malformed storage', () => {
    writeCompanyColumnVisibility('ws-1', { 'cp:fit': true });
    expect(readCompanyColumnVisibility('ws-1')).toEqual({ 'cp:fit': true });
    expect(readCompanyColumnVisibility('ws-2')).toEqual({});
    window.localStorage.setItem('helpin.crm.companies.columns.v1:ws-3', '{"cp:fit":"yes","name":false}');
    expect(readCompanyColumnVisibility('ws-3')).toEqual({ name: false });
    window.localStorage.setItem('helpin.crm.companies.columns.v1:ws-4', 'not json');
    expect(readCompanyColumnVisibility('ws-4')).toEqual({});
  });
});
