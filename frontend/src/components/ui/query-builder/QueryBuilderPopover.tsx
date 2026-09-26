import { memo, useMemo, useState, type ComponentProps, type ReactNode } from 'react';
import { Cancel01Icon, FilterHorizontalIcon, PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  getDefaultOperator,
  getOperatorsForField,
  isQueryFilterRuleComplete,
  normalizeQueryFilterGroup,
  operatorUsesRange,
  operatorUsesValue,
  QUERY_FILTER_OPERATOR_LABELS,
  type QueryBuilderFieldDefinition,
  type QueryFilterGroup,
  type QueryFilterOperator,
  type QueryFilterRule,
} from '@/lib/queryBuilder';
import { cn } from '@/lib/utils';

interface QueryBuilderPopoverProps {
  fields: QueryBuilderFieldDefinition[];
  value?: QueryFilterGroup;
  onApply: (group?: QueryFilterGroup) => void;
  triggerLabel?: string;
  triggerVariant?: ComponentProps<typeof Button>['variant'];
  triggerClassName?: string;
  allowOr?: boolean;
  maxRules?: number;
  presentation?: 'default' | 'quiet';
  emptyDescription?: string;
  renderValue?: (rule: QueryFilterRule, onChange: (value: string) => void) => ReactNode;
}

interface DraftRule extends QueryFilterRule {
  id: string;
}

const QueryBuilderRuleRow = memo(function QueryBuilderRuleRow({
  index,
  rule,
  field,
  fields,
  canRemove,
  onFieldChange,
  onOperatorChange,
  onValueChange,
  onRangeValueChange,
  onRemove,
  logic,
  renderValue,
}: {
  index: number;
  rule: DraftRule;
  field?: QueryBuilderFieldDefinition;
  fields: QueryBuilderFieldDefinition[];
  canRemove: boolean;
  onFieldChange: (fieldKey: string) => void;
  onOperatorChange: (operator: QueryFilterOperator) => void;
  onValueChange: (value: string) => void;
  onRangeValueChange: (position: 0 | 1, value: string) => void;
  onRemove: () => void;
  logic: 'and' | 'or';
  renderValue?: QueryBuilderPopoverProps['renderValue'];
}) {
  const operators = field ? getOperatorsForField(field) : [];
  const showValue = operatorUsesValue(rule.operator);
  const showRange = operatorUsesRange(rule.operator);
  const primaryLabel = index === 0 ? 'Where' : logic;

  return (
    <div className="grid gap-1.5 md:grid-cols-[48px_minmax(0,1fr)_160px_minmax(0,1fr)_24px] md:items-center">
      <div className="text-[11px] text-muted-foreground">{primaryLabel}</div>

      <Select value={rule.field} onValueChange={onFieldChange}>
        <SelectTrigger aria-label={`Filter ${index + 1} field`} className="h-8 w-full text-[11px]">
          <SelectValue placeholder="Field" />
        </SelectTrigger>
        <SelectContent>
          {fields.map((entry) => (
            <SelectItem key={entry.field} value={entry.field}>
              {entry.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select value={rule.operator} onValueChange={(value) => onOperatorChange(value as QueryFilterOperator)}>
        <SelectTrigger aria-label={`Filter ${index + 1} operator`} className="h-8 w-full text-[11px]">
          <SelectValue placeholder="Operator" />
        </SelectTrigger>
        <SelectContent>
          {operators.map((operator) => (
            <SelectItem key={operator} value={operator}>
              {QUERY_FILTER_OPERATOR_LABELS[operator]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="min-w-0">
        {!showValue ? (
          <div className="h-8 rounded-md border border-dashed border-border/70 bg-muted/30" />
        ) : renderValue?.(rule, onValueChange) ?? ((field?.type === 'date' || field?.type === 'number') && showRange ? (
          <div className="grid grid-cols-2 gap-1.5">
            <Input
              type={field?.type === 'number' ? 'number' : 'date'}
              step={field?.type === 'number' ? 'any' : undefined}
              aria-label={`${field?.label ?? 'Filter'} minimum`}
              value={rule.values?.[0] ?? ''}
              onChange={(event) => onRangeValueChange(0, event.target.value)}
              className="h-8 text-[11px]"
            />
            <Input
              type={field?.type === 'number' ? 'number' : 'date'}
              step={field?.type === 'number' ? 'any' : undefined}
              aria-label={`${field?.label ?? 'Filter'} maximum`}
              value={rule.values?.[1] ?? ''}
              onChange={(event) => onRangeValueChange(1, event.target.value)}
              className="h-8 text-[11px]"
            />
          </div>
        ) : field?.type === 'date' ? (
          <Input
            type="date"
            value={rule.value ?? ''}
            onChange={(event) => onValueChange(event.target.value)}
            className="h-8 text-[11px]"
          />
        ) : field?.options?.length ? (
          <Select value={rule.value ?? '__empty__'} onValueChange={(value) => onValueChange(value === '__empty__' ? '' : value)}>
            <SelectTrigger className="h-8 w-full text-[11px]">
              <SelectValue placeholder={field.placeholder ?? 'Select value'} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__empty__">{field.placeholder ?? 'Select value'}</SelectItem>
              {field.options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Input
            type={field?.type === 'number' ? 'number' : 'text'}
            step={field?.type === 'number' ? 'any' : undefined}
            aria-label={`${field?.label ?? 'Filter'} value`}
            value={rule.value ?? ''}
            onChange={(event) => onValueChange(event.target.value)}
            placeholder={field?.placeholder ?? 'Enter value'}
            className="h-8 text-[11px]"
          />
        ))}
      </div>

      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-6 w-6 self-start md:self-center"
        onClick={onRemove}
        aria-label={`Remove filter ${index + 1}`}
        disabled={!canRemove}
      >
        <Cancel01Icon className="h-3 w-3" />
      </Button>
    </div>
  );
});

export function QueryBuilderPopover({
  fields,
  value,
  onApply,
  triggerLabel = 'Filter',
  triggerVariant = 'outline',
  triggerClassName,
  allowOr = false,
  maxRules = Infinity,
  presentation = 'default',
  emptyDescription = 'Add one or more conditions to filter contacts.',
  renderValue,
}: QueryBuilderPopoverProps) {
  const [open, setOpen] = useState(false);
  const [logic, setLogic] = useState<'and' | 'or'>(allowOr ? value?.logic ?? 'and' : 'and');
  const [draftRules, setDraftRules] = useState<DraftRule[]>(() => toDraftRules(value?.rules, fields));

  const changeOpen = (next: boolean) => {
    if (next) {
      const saved = toDraftRules(value?.rules, fields);
      setDraftRules(saved.length ? saved : fields[0] ? [createDraftRule(fields[0])] : []);
      setLogic(allowOr ? value?.logic ?? 'and' : 'and');
    }
    setOpen(next);
  };

  const activeCount = value?.rules.length ?? 0;
  const fieldMap = useMemo(
    () => new Map(fields.map((field) => [field.field, field])),
    [fields],
  );
  const hasDraftChanges = useMemo(() => {
    const normalizedDraft = normalizeQueryFilterGroup({
      logic,
      rules: draftRules.map(stripDraftRule),
    });
    const normalizedValue = normalizeQueryFilterGroup(value);
    return JSON.stringify(normalizedDraft ?? null) !== JSON.stringify(normalizedValue ?? null);
  }, [draftRules, logic, value]);
  const hasIncompleteRules = useMemo(
    () => draftRules.some((rule) => !isQueryFilterRuleComplete(stripDraftRule(rule))),
    [draftRules],
  );

  const addRule = () => {
    const nextField = fields[0];
    if (!nextField) return;
    setDraftRules((current) => [...current, createDraftRule(nextField)]);
  };

  const clearRules = () => {
    setDraftRules([]);
    onApply(undefined);
    setOpen(false);
  };

  return (
    <div className="flex items-center gap-1.5">
      <Popover open={open} onOpenChange={changeOpen}>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant={triggerVariant}
            size="sm"
            className={cn('h-6 gap-1.5 px-2 text-[11px]', triggerClassName)}
          >
            <FilterHorizontalIcon className="h-3 w-3" />
            {triggerLabel}
            {activeCount > 0 && presentation === 'quiet' && <span className="text-quiet-text-tertiary">{activeCount}</span>}
            {activeCount > 0 && presentation !== 'quiet' && (
              <Badge variant="secondary" className="h-4 min-w-4 rounded-full px-1 text-[10px]">
                {activeCount}
              </Badge>
            )}
          </Button>
        </PopoverTrigger>
        <PopoverContent
          align="start"
          className={cn('w-[min(680px,calc(100vw-2rem))] max-w-[calc(100vw-2rem)] gap-0 p-0', presentation === 'quiet' && '[&_input]:rounded-none [&_input]:border-0 [&_input]:border-b [&_input]:border-quiet-field [&_input]:bg-transparent [&_input]:shadow-none [&_input]:focus-visible:ring-0 [&_input]:focus-visible:border-b-2 [&_[data-slot=select-trigger]]:rounded-none [&_[data-slot=select-trigger]]:border-0 [&_[data-slot=select-trigger]]:border-b [&_[data-slot=select-trigger]]:border-quiet-field [&_[data-slot=select-trigger]]:bg-transparent [&_[data-slot=select-trigger]]:shadow-none')}
        >
          <div className="border-b border-border/70 px-3.5 py-2.5">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium">Filters</span>
                {activeCount > 0 && presentation === 'quiet' && <span className="text-xs text-quiet-text-tertiary">{activeCount} conditions</span>}
                {activeCount > 0 && presentation !== 'quiet' && (
                  <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                    {activeCount} active
                  </Badge>
                )}
              </div>
              {activeCount > 0 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 px-2 text-[11px] text-muted-foreground"
                  onClick={clearRules}
                >
                  Clear all filters
                </Button>
              )}
            </div>
          </div>

          <div className="space-y-2.5 px-3.5 py-2.5">
            {allowOr && <Select value={logic} onValueChange={(value) => setLogic(value as 'and' | 'or')}><SelectTrigger aria-label="Match conditions" variant="ghost" size="sm"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="and">Match all conditions</SelectItem><SelectItem value="or">Match any condition</SelectItem></SelectContent></Select>}
            {draftRules.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 px-3 py-4 text-xs text-muted-foreground">
                {emptyDescription}
              </div>
            ) : (
              draftRules.map((rule, index) => (
                <QueryBuilderRuleRow
                  key={rule.id}
                  index={index}
                  rule={rule}
                  field={fieldMap.get(rule.field)}
                  fields={fields}
                  canRemove={draftRules.length > 1}
                  logic={logic}
                  renderValue={renderValue}
                  onFieldChange={(fieldKey) => {
                    const nextField = fieldMap.get(fieldKey);
                    if (!nextField) return;
                    setDraftRules((current) => current.map((entry) => (
                      entry.id === rule.id ? createDraftRule(nextField, entry.id) : entry
                    )));
                  }}
                  onOperatorChange={(operator) => {
                    setDraftRules((current) => current.map((entry) => {
                      if (entry.id !== rule.id) {
                        return entry;
                      }
                      return {
                        ...entry,
                        operator,
                        value: undefined,
                        values: operatorUsesRange(operator) ? ['', ''] : undefined,
                      };
                    }));
                  }}
                  onValueChange={(value) => {
                    setDraftRules((current) => current.map((entry) => (
                      entry.id === rule.id ? { ...entry, value } : entry
                    )));
                  }}
                  onRangeValueChange={(position, value) => {
                    setDraftRules((current) => current.map((entry) => {
                      if (entry.id !== rule.id) {
                        return entry;
                      }
                      const nextValues = [...(entry.values ?? ['', ''])];
                      nextValues[position] = value;
                      return { ...entry, values: nextValues };
                    }));
                  }}
                  onRemove={() => {
                    setDraftRules((current) => current.filter((entry) => entry.id !== rule.id));
                  }}
                />
              ))
            )}

            <div className="flex flex-wrap items-center justify-between gap-2.5 border-t border-border/60 pt-3">
              <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={addRule} disabled={draftRules.length >= maxRules}>
                <PlusSignIcon className="mr-1.5 h-3 w-3" />
                Add filter
              </Button>

              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 px-2 text-[11px]"
                  onClick={() => {
                    setDraftRules(toDraftRules(value?.rules, fields));
                    setOpen(false);
                  }}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  size="sm"
                  className="h-6 px-2 text-[11px]"
                  disabled={!hasDraftChanges || hasIncompleteRules}
                  onClick={() => {
                    onApply(normalizeQueryFilterGroup({
                      logic,
                      rules: draftRules.map(stripDraftRule),
                    }));
                    setOpen(false);
                  }}
                >
                  Apply filters
                </Button>
              </div>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}

function createDraftRule(field: QueryBuilderFieldDefinition, id = createRuleId()): DraftRule {
  const operator = getDefaultOperator(field);
  return {
    id,
    field: field.field,
    operator,
    value: undefined,
    values: operatorUsesRange(operator) ? ['', ''] : undefined,
  };
}

function toDraftRules(rules: QueryFilterRule[] | undefined, fields: QueryBuilderFieldDefinition[]): DraftRule[] {
  if (!rules?.length) {
    return [];
  }

  const fieldMap = new Map(fields.map((field) => [field.field, field]));
  const draftRules: Array<DraftRule | null> = rules.map((rule) => {
      const field = fieldMap.get(rule.field);
      if (!field) {
        return null;
      }
      return {
        id: createRuleId(),
        field: rule.field,
        operator: rule.operator,
        value: rule.value,
        values: rule.values ? [...rule.values] : undefined,
      } satisfies DraftRule;
    });

  return draftRules.filter((rule): rule is DraftRule => rule !== null);
}

function stripDraftRule(rule: DraftRule): QueryFilterRule {
  return {
    field: rule.field,
    operator: rule.operator,
    value: rule.value,
    values: rule.values,
  };
}

function createRuleId(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2);
}
