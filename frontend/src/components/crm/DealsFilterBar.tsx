import { QueryBuilderPopover } from '@/components/ui/query-builder/QueryBuilderPopover';
import { Button } from '@/components/ui/button';
import { Cancel01Icon } from '@/lib/icons';
import {
  QUERY_FILTER_OPERATOR_LABELS,
  operatorUsesValue,
  operatorUsesRange,
  type QueryFilterGroup,
  type QueryBuilderFieldDefinition,
} from '@/lib/queryBuilder';
type Props = {
  fields: QueryBuilderFieldDefinition[];
  value?: QueryFilterGroup;
  onChange: (group?: QueryFilterGroup) => void;
};
export function DealsFilterBar({ fields, value, onChange }: Props) {
  return (
    <QueryBuilderPopover
      fields={fields}
      value={value}
      onApply={onChange}
      triggerLabel="Filters"
      triggerVariant="ghost"
      triggerClassName="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground"
    />
  );
}
export function DealsActiveFilterBar({ fields, value, onChange }: Props) {
  if (!value?.rules.length) return null;
  return (
    <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-1.5 px-3 py-1.5">
      {value.rules.map((rule, index) => {
        const field = fields.find((f) => f.field === rule.field);
        const raw = operatorUsesValue(rule.operator)
          ? operatorUsesRange(rule.operator)
            ? (rule.values ?? [])
            : [rule.value ?? '']
          : [];
        const label = raw
          .map((v) => field?.options?.find((o) => o.value === v)?.label ?? v)
          .join(' and ');
        return (
          <div
            key={`${rule.field}-${index}`}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs"
          >
            <span className="font-medium text-muted-foreground">
              {field?.label ?? rule.field}
            </span>
            <span className="text-muted-foreground/60">
              {QUERY_FILTER_OPERATOR_LABELS[rule.operator]}
            </span>
            {label && (
              <span className="max-w-48 truncate text-foreground">{label}</span>
            )}
            <button
              type="button"
              className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
              aria-label={`Remove ${field?.label ?? rule.field} filter`}
              onClick={() => {
                const rules = value.rules.filter((_, i) => i !== index);
                onChange(rules.length ? { ...value, rules } : undefined);
              }}
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
        onClick={() => onChange(undefined)}
      >
        Clear all
      </Button>
    </div>
  );
}
