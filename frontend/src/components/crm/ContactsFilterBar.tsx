import { useMemo } from 'react';
import { QueryBuilderPopover } from '@/components/ui/query-builder/QueryBuilderPopover';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import { buildCRMContactQueryFields } from '@/lib/crmContactQueryBuilder';
import { QUERY_FILTER_OPERATOR_LABELS, operatorUsesRange, operatorUsesValue, type QueryBuilderFieldDefinition, type QueryFilterRule } from '@/lib/queryBuilder';
import { Button } from '@/components/ui/button';
import { Cancel01Icon } from '@/lib/icons';
import type { AssignableMember } from '@/lib/types';

interface ContactsFilterBarProps {
  assignableMembers: AssignableMember[];
}

export function ContactsFilterBar({ assignableMembers }: ContactsFilterBarProps) {
  const { filterGroup, setFilterGroup } = useContactsSearchParams();
  const fields = useMemo(
    () => buildCRMContactQueryFields(assignableMembers),
    [assignableMembers],
  );

  return (
    <QueryBuilderPopover
      fields={fields}
      value={filterGroup}
      onApply={setFilterGroup}
      triggerLabel="Filters"
      triggerVariant="ghost"
      triggerClassName="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground"
    />
  );
}

function ruleValueLabel(rule: QueryFilterRule, field?: QueryBuilderFieldDefinition): string | null {
  if (!operatorUsesValue(rule.operator)) return null;
  const values = operatorUsesRange(rule.operator) ? (rule.values ?? []) : [rule.value ?? ''];
  const labels = values.filter(Boolean).map((value) => (
    field?.options?.find((option) => option.value === value)?.label ?? value
  ));
  return labels.length > 0 ? labels.join(' and ') : null;
}

export function ContactsActiveFilterBar({ assignableMembers }: ContactsFilterBarProps) {
  const { search, filterGroup, setFilterGroup, setParam, setParams } = useContactsSearchParams();
  const fields = useMemo(
    () => buildCRMContactQueryFields(assignableMembers),
    [assignableMembers],
  );
  const fieldMap = useMemo(
    () => new Map(fields.map((field) => [field.field, field])),
    [fields],
  );
  const queryRules = filterGroup?.rules ?? [];
  const legacyRules = [
    search.stage ? { key: 'stage' as const, field: 'lifecycle_stage', value: search.stage } : null,
    search.status ? { key: 'status' as const, field: 'lead_status', value: search.status } : null,
    search.owner ? { key: 'owner' as const, field: 'owner_member_id', value: search.owner } : null,
  ].filter((entry): entry is NonNullable<typeof entry> => entry !== null);

  if (queryRules.length === 0 && legacyRules.length === 0) return null;

  const removeRule = (index: number) => {
    const rules = queryRules.filter((_, ruleIndex) => ruleIndex !== index);
    setFilterGroup(rules.length > 0 ? { logic: filterGroup?.logic ?? 'and', rules } : undefined);
  };

  return (
    <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-1.5 px-3 py-1.5">
      {queryRules.map((rule, index) => {
        const field = fieldMap.get(rule.field);
        const valueLabel = ruleValueLabel(rule, field);
        return (
          <div key={`${rule.field}-${rule.operator}-${index}`} className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
            <span className="font-medium text-muted-foreground">{field?.label ?? rule.field}</span>
            <span className="text-muted-foreground/60">{QUERY_FILTER_OPERATOR_LABELS[rule.operator]}</span>
            {valueLabel ? <span className="max-w-48 truncate text-foreground">{valueLabel}</span> : null}
            <button
              type="button"
              onClick={() => removeRule(index)}
              className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
              aria-label={`Remove ${field?.label ?? rule.field} filter`}
            >
              <Cancel01Icon className="h-3 w-3" />
            </button>
          </div>
        );
      })}
      {legacyRules.map((rule) => {
        const field = fieldMap.get(rule.field);
        const valueLabel = field?.options?.find((option) => option.value === rule.value)?.label ?? rule.value;
        return (
          <div key={rule.key} className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
            <span className="font-medium text-muted-foreground">{field?.label ?? rule.field}</span>
            <span className="text-muted-foreground/60">is</span>
            <span className="max-w-48 truncate text-foreground">{valueLabel}</span>
            <button
              type="button"
              onClick={() => setParam(rule.key, undefined)}
              className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
              aria-label={`Remove ${field?.label ?? rule.field} filter`}
            >
              <Cancel01Icon className="h-3 w-3" />
            </button>
          </div>
        );
      })}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-[10px] text-muted-foreground"
        onClick={() => setParams({ filters: undefined, stage: undefined, status: undefined, owner: undefined })}
      >
        Clear all
      </Button>
    </div>
  );
}
