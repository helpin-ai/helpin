// @vitest-environment jsdom

import { afterEach, describe, expect, it } from 'vitest';

import {
  buildCompanyFilterDefinitions,
  companyFilterGroupToState,
  companyFiltersToGroup,
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
import type { CustomFieldDefinition } from '../customFields';

function definedField(field: Partial<CustomFieldDefinition>): CustomFieldDefinition {
  return { id: field.key ?? 'id', object_type: 'crm_company', key: 'key', label: '', display_label: '', description: '', field_type: 'text', options: [], position: 0, ...field };
}

describe('company filter values', () => {
  it('turns selections into one is_any_of rule per field and back', () => {
    const values = {
      [customPropertyFilterField('fit')]: ['yes', 'maybe'],
      industry: ['Software'],
      owner_member_id: [],
    };
    const group = companyFiltersToGroup({ values, conditions: {}, modes: {} });

    expect(group).toEqual({
      logic: 'and',
      rules: [
        { field: 'custom_properties.fit', operator: 'is_any_of', values: ['yes', 'maybe'] },
        { field: 'industry', operator: 'is_any_of', values: ['Software'] },
      ],
    });
    expect(companyFilterGroupToState(group).values).toEqual({
      'custom_properties.fit': ['yes', 'maybe'],
      industry: ['Software'],
    });
  });

  it('survives URL serialization, keeping the value list', () => {
    const group = companyFiltersToGroup({
      values: { 'custom_properties.support_tools': ['zendesk'] },
      conditions: { 'custom_properties.employees_est': { operator: 'between', values: ['10', '50'] } },
      modes: {},
    });
    const parsed = companyFilterGroupToState(parseQueryFilterGroup(serializeQueryFilterGroup(group)));
    expect(parsed.values).toEqual({ 'custom_properties.support_tools': ['zendesk'] });
    expect(parsed.conditions).toEqual({ 'custom_properties.employees_est': { operator: 'between', values: ['10', '50'] } });
  });

  it('returns no group without selections and ignores unsupported operators when reading', () => {
    expect(companyFiltersToGroup({ values: { industry: [] }, conditions: {}, modes: {} })).toBeUndefined();
    expect(companyFilterGroupToState({ logic: 'and', rules: [{ field: 'name', operator: 'contains', value: 'acme' }] }))
      .toEqual({ values: {}, conditions: {}, modes: {} });
  });

  it('maps number conditions to numeric rules, with equals as a one-value range', () => {
    const field = customPropertyFilterField('employees_est');
    const rule = (condition: Parameters<typeof companyFiltersToGroup>[0]['conditions'][string]) =>
      companyFiltersToGroup({ values: {}, conditions: { [field]: condition }, modes: {} })?.rules[0];

    expect(rule({ operator: 'gt', value: ' 50 ' })).toEqual({ field, operator: 'gt', value: '50' });
    expect(rule({ operator: 'equals', value: '40' })).toEqual({ field, operator: 'between', values: ['40', '40'] });
    expect(rule({ operator: 'is_empty' })).toEqual({ field, operator: 'is_empty' });
    expect(companyFilterGroupToState({ logic: 'and', rules: [{ field, operator: 'between', values: ['40', '40'] }] }, new Set([field])).conditions)
      .toEqual({ [field]: { operator: 'equals', value: '40' } });
  });

  it('leaves incomplete number conditions out of the request', () => {
    const field = customPropertyFilterField('employees_est');
    expect(companyFiltersToGroup({ values: {}, conditions: { [field]: { operator: 'gt' } }, modes: {} })).toBeUndefined();
    expect(companyFiltersToGroup({ values: {}, conditions: { [field]: { operator: 'between', values: ['10', ''] } }, modes: {} })).toBeUndefined();
  });

  it('maps option operators to is_none_of and empty checks, and back', () => {
    const fit = customPropertyFilterField('fit');
    const group = companyFiltersToGroup({
      values: { [fit]: ['no'], industry: ['Retail'] },
      conditions: {},
      modes: { [fit]: 'is_none_of', owner_member_id: 'is_empty', industry: 'is_not_empty' },
    });
    expect(group?.rules).toEqual([
      { field: fit, operator: 'is_none_of', values: ['no'] },
      { field: 'industry', operator: 'is_not_empty' },
      { field: 'owner_member_id', operator: 'is_empty' },
    ]);
    expect(companyFilterGroupToState(group)).toEqual({
      values: { [fit]: ['no'] },
      conditions: {},
      modes: { [fit]: 'is_none_of', industry: 'is_not_empty', owner_member_id: 'is_empty' },
    });
  });

  it('reads an empty check as a number condition only for number fields', () => {
    const employees = customPropertyFilterField('employees_est');
    const group = { logic: 'and' as const, rules: [{ field: employees, operator: 'is_empty' as const }] };
    expect(companyFilterGroupToState(group, new Set([employees])).conditions).toEqual({ [employees]: { operator: 'is_empty' } });
    expect(companyFilterGroupToState(group).modes).toEqual({ [employees]: 'is_empty' });
  });

  it('leaves an is any of / is none of filter without values out of the request', () => {
    expect(companyFiltersToGroup({ values: {}, conditions: {}, modes: { industry: 'is_none_of' } })).toBeUndefined();
  });

  it('toggles and removes filters without mutating the input', () => {
    const start = { industry: ['Software'] };
    expect(toggleCompanyFilterValue(start, 'industry', 'Retail')).toEqual({ industry: ['Software', 'Retail'] });
    expect(toggleCompanyFilterValue(start, 'industry', 'Software')).toEqual({ industry: [] });
    const state = {
      values: start,
      conditions: { 'custom_properties.n': { operator: 'gt' as const, value: '1' } },
      modes: { industry: 'is_none_of' as const },
    };
    expect(removeCompanyFilter(state, 'industry')).toEqual({ values: {}, conditions: state.conditions, modes: {} });
    expect(removeCompanyFilter(state, 'custom_properties.n')).toEqual({ values: start, conditions: {}, modes: state.modes });
    expect(start).toEqual({ industry: ['Software'] });
  });
});

describe('buildCompanyFilterDefinitions', () => {
  const options: CRMCompanyFilterOptions = {
    industries: [{ value: 'Software', count: 3 }],
    custom_properties: [
      { key: 'is_b2b', count: 2, value_type: 'boolean', values: [{ value: 'true', count: 2 }, { value: 'false', count: 1 }], has_more_values: false, numeric_count: 0 },
      { key: 'support_tools', count: 2, value_type: 'list', values: [{ value: 'zendesk', count: 2 }], has_more_values: false, numeric_count: 0 },
      { key: 'employees_est', count: 5, value_type: 'mixed', values: [{ value: '45', count: 1 }], has_more_values: false, numeric_count: 3, numeric_min: 8, numeric_max: 450 },
      { key: 'size_band', count: 5, value_type: 'text', values: [{ value: '11-50', count: 3 }], has_more_values: false, numeric_count: 1 },
    ],
    scanned_companies: 3,
    truncated: false,
  };
  const members = [{ id: 'm1', display_name: 'Ada Lovelace', email: 'ada@example.com' }] as AssignableMember[];

  it('puts Owner first, then Industry, then custom properties with readable labels', () => {
    const definitions = buildCompanyFilterDefinitions(options, members);
    expect(definitions.map((definition) => definition.label)).toEqual(['Owner', 'Industry', 'Is b2b', 'Support tools', 'Employees est', 'Size band']);
    expect(definitions[0].options).toEqual([{ value: 'm1', label: 'Ada Lovelace' }]);
    expect(definitions.filter((definition) => definition.kind !== 'number').every((definition) => definition.optionModes)).toBe(true);
    expect(definitions[2].key).toBe('custom_properties.is_b2b');
    expect(definitions[2].options.map((option) => option.label)).toEqual(['Yes', 'No']);
    expect(definitions[3].options).toEqual([{ value: 'zendesk', label: 'zendesk' }]);
  });

  it('filters mostly numeric keys as numbers and notes skipped values', () => {
    const definitions = buildCompanyFilterDefinitions(options, members);
    const employees = definitions.find((definition) => definition.key === 'custom_properties.employees_est');
    const sizeBand = definitions.find((definition) => definition.key === 'custom_properties.size_band');
    expect(employees).toMatchObject({ kind: 'number', note: '2 not numeric', options: [] });
    expect(sizeBand?.kind).toBeUndefined();
  });

  it('lists defined fields first, typed by definition, and groups the rest as not defined', () => {
    const fields = [
      definedField({ key: 'renews_on', display_label: 'Renews on', field_type: 'date' }),
      definedField({ key: 'size_band', display_label: 'Size', field_type: 'select', options: [{ value: '11-50', label: 'Small' }] }),
    ];
    const definitions = buildCompanyFilterDefinitions(options, members, fields);
    expect(definitions.map((definition) => [definition.label, definition.group])).toEqual([
      ['Owner', undefined], ['Industry', undefined], ['Renews on', 'Fields'], ['Size', 'Fields'],
      ['Is b2b', 'Not defined'], ['Support tools', 'Not defined'], ['Employees est', 'Not defined'],
    ]);
    expect(definitions[2].kind).toBe('date');
    expect(definitions[3]).toMatchObject({ options: [{ value: '11-50', label: 'Small' }], optionModes: true });
  });

  it('turns date conditions into date rules and back', () => {
    const field = customPropertyFilterField('renews_on');
    const dateFields = new Set([field]);
    const state = { values: {}, modes: {}, conditions: { [field]: { operator: 'lt' as const, value: '2026-11-01' } } };
    const group = companyFiltersToGroup(state, dateFields);
    expect(group?.rules).toEqual([{ field, operator: 'before', value: '2026-11-01' }]);
    expect(companyFilterGroupToState(group, dateFields).conditions[field]).toEqual({ operator: 'lt', value: '2026-11-01' });
    const range = companyFiltersToGroup({ values: {}, modes: {}, conditions: { [field]: { operator: 'between', values: ['2026-10-01', '2026-10-31'] } } }, dateFields);
    expect(range?.rules[0]).toEqual({ field, operator: 'between', values: ['2026-10-01', '2026-10-31'] });
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
