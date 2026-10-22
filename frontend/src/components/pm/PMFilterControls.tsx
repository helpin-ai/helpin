import { useState, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import {
  PM_FILTER_NUMBER_OPERATORS,
  PM_FILTER_OPTION_MODES,
  optionModeUsesValues,
  type PMFilterCondition,
  type PMFilterConditions,
  type PMFilterNumberOperator,
  type PMFilterOptionMode,
  type PMFilterOptionModes,
} from '@/lib/pmFilterConditions';

export type {
  PMFilterCondition,
  PMFilterConditions,
  PMFilterNumberOperator,
  PMFilterOptionMode,
  PMFilterOptionModes,
} from '@/lib/pmFilterConditions';

import {
  ArrowLeft02Icon,
  Cancel01Icon,
  FilterHorizontalIcon,
  Tick01Icon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  QuietDropdownOptions,
  QuietDropdownEmpty,
  QuietDropdownGroup,
  QuietDropdownItem,
} from '@/components/design-system/quiet-dropdown';
import {
  QuietDropdownRoot as Popover,
  QuietDropdownContent as PopoverContent,
  QuietDropdownTrigger as PopoverTrigger,
} from '@/components/design-system/quiet-dropdown';

export interface PMFilterOption {
  value: string;
  label: string;
  icon?: ReactNode;
  labelClassName?: string;
}

export interface PMFilterDefinition<K extends string> {
  key: K;
  label: string;
  options: PMFilterOption[];
  singleSelect?: boolean;
  searchableValues?: boolean;
  contentClassName?: string;
  /**
   * options (default) picks values from a list; number compares against a
   * typed number with the operators in PM_FILTER_NUMBER_OPERATORS.
   */
  kind?: 'options' | 'number';
  /** Short muted note shown on the applied pill, such as skipped records. */
  note?: string;
  /**
   * Lets an options filter choose is any of, is none of, has a value, or is
   * empty instead of the fixed "is".
   */
  optionModes?: boolean;
}

const OPTION_MODE_LABELS: Record<PMFilterOptionMode, string> = {
  is_any_of: 'is',
  is_none_of: 'is not',
  is_not_empty: 'has a value',
  is_empty: 'is empty',
};

export type PMFilterValues<K extends string> = Partial<Record<K, string[]>>;

const NUMBER_OPERATOR_LABELS: Record<PMFilterNumberOperator, string> = {
  equals: '=',
  gt: '>',
  gte: '≥',
  lt: '<',
  lte: '≤',
  between: 'between',
  is_not_empty: 'has a value',
  is_empty: 'is empty',
};

const NUMBER_OPERATOR_NAMES: Record<PMFilterNumberOperator, string> = {
  equals: 'equals',
  gt: 'greater than',
  gte: 'at least',
  lt: 'less than',
  lte: 'at most',
  between: 'between',
  is_not_empty: 'has a value',
  is_empty: 'is empty',
};

function PMFilterValueSelect<K extends string>({
  definition,
  selected,
  onToggle,
}: {
  definition: PMFilterDefinition<K>;
  selected: string[];
  onToggle: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const selectedLabels = selected.map(
    (value) =>
      definition.options.find((option) => option.value === value)?.label ??
      value,
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded border border-border bg-background px-1.5 py-0.5 text-xs transition-colors hover:bg-accent"
        >
          {selectedLabels.length === 0
            ? 'Choose value'
            : selectedLabels.length === 1
              ? selectedLabels[0]
              : `${selectedLabels.length} selected`}
        </button>
      </PopoverTrigger>
      <PopoverContent
        className={cn(definition.searchableValues ? 'w-80' : 'w-52', 'p-0', definition.contentClassName)}
        align="start"
      >
        <QuietDropdownOptions
          searchPlaceholder={`Search ${definition.label.toLowerCase()}...`}
        >
          <QuietDropdownEmpty>No results.</QuietDropdownEmpty>
          <QuietDropdownGroup>
            {definition.options.map((option) => {
              const isSelected = selected.includes(option.value);
              return (
                <QuietDropdownItem
                  key={option.value}
                  value={option.value}
                  keywords={[option.label]}
                  onSelect={() => onToggle(option.value)}
                >
                  <div
                    className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}
                  >
                    {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                  </div>
                  {option.icon ? (
                    <span className="mr-1.5 shrink-0">{option.icon}</span>
                  ) : null}
                  <span className={cn("min-w-0 flex-1 truncate", option.labelClassName)}>
                    {option.label}
                  </span>
                </QuietDropdownItem>
              );
            })}
          </QuietDropdownGroup>
        </QuietDropdownOptions>
      </PopoverContent>
    </Popover>
  );
}

export function PMFilterPill<K extends string>({
  definition,
  selected,
  onToggle,
  onRemove,
  mode = 'is_any_of',
  onModeChange,
}: {
  definition: PMFilterDefinition<K>;
  selected: string[];
  onToggle: (value: string) => void;
  onRemove: () => void;
  mode?: PMFilterOptionMode;
  onModeChange?: (mode: PMFilterOptionMode) => void;
}) {
  return (
    <div className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">
        {definition.label}
      </span>
      {definition.optionModes && onModeChange ? (
        <PMOperatorSelect
          label={definition.label}
          options={PM_FILTER_OPTION_MODES.map((option) => ({ value: option, label: OPTION_MODE_LABELS[option], name: OPTION_MODE_LABELS[option] }))}
          value={mode}
          onChange={onModeChange}
        />
      ) : (
        <span className="text-muted-foreground/60">is</span>
      )}
      {optionModeUsesValues(mode) ? (
        <PMFilterValueSelect
          definition={definition}
          selected={selected}
          onToggle={onToggle}
        />
      ) : null}
      <button
        type="button"
        onClick={onRemove}
        className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
        aria-label={`Remove ${definition.label} filter`}
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </div>
  );
}

export function PMFilterTrigger<K extends string>({
  definitions,
  values,
  visibleKeys,
  activeCount,
  onAdd,
  onToggle,
}: {
  definitions: PMFilterDefinition<K>[];
  values: PMFilterValues<K>;
  visibleKeys: Set<K>;
  activeCount: number;
  onAdd: (key: K) => void;
  onToggle: (key: K, value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<K | null>(null);
  const available = definitions.filter(
    (definition) =>
      !visibleKeys.has(definition.key) && (definition.kind === 'number' || definition.options.length > 0),
  );
  const selectedDefinition = selectedKey
    ? definitions.find((definition) => definition.key === selectedKey)
    : undefined;
  const canChooseFilter = available.length > 0 || Boolean(selectedDefinition);

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    if (!nextOpen) setSelectedKey(null);
  };

  if (!canChooseFilter) {
    return (
      <Button
        variant="ghost"
        size="sm"
        className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground"
        disabled
      >
        <span className="inline-flex items-center gap-1">
          <FilterHorizontalIcon className="h-3.5 w-3.5" />
          Filters
        </span>
        <Badge
          variant="secondary"
          className="ml-0.5 rounded-full px-1.5 py-0 text-[10px]"
        >
          {activeCount}
        </Badge>
      </Button>
    );
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground"
        >
          <span className="inline-flex items-center gap-1">
            <FilterHorizontalIcon className="h-3.5 w-3.5" />
            Filters
          </span>
          <Badge
            variant="secondary"
            className={`rounded-full px-1.5 py-0 text-[10px] transition-opacity ${activeCount > 0 ? 'opacity-100' : 'opacity-0'}`}
          >
            {activeCount || 0}
          </Badge>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className={cn(selectedDefinition ? (selectedDefinition.searchableValues ? 'w-80' : 'w-52') : 'w-48', 'p-0', selectedDefinition?.contentClassName)}
        align="start"
      >
        {selectedDefinition ? (
          <QuietDropdownOptions
            key={selectedDefinition.key}
            searchPlaceholder={`Search ${selectedDefinition.label.toLowerCase()}...`}
            header={
              <div className="flex items-center gap-1 border-b border-border/70 px-1.5 py-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  aria-label="Back to filter fields"
                  onClick={() => setSelectedKey(null)}
                >
                  <ArrowLeft02Icon className="h-3.5 w-3.5" />
                </Button>
                <span className="truncate text-xs font-medium">
                  {selectedDefinition.label}
                </span>
              </div>
            }
          >
            <QuietDropdownEmpty>No results.</QuietDropdownEmpty>
            <QuietDropdownGroup>
              {selectedDefinition.options.map((option) => {
                const isSelected =
                  values[selectedDefinition.key]?.includes(option.value) ??
                  false;
                return (
                  <QuietDropdownItem
                    key={option.value}
                    value={option.value}
                    keywords={[option.label]}
                    onSelect={() => {
                      onToggle(selectedDefinition.key, option.value);
                      if (selectedDefinition.singleSelect) {
                        setOpen(false);
                        setSelectedKey(null);
                      }
                    }}
                  >
                    <div
                      className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}
                    >
                      {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                    </div>
                    {option.icon ? (
                      <span className="mr-1.5 shrink-0">{option.icon}</span>
                    ) : null}
                    <span className={cn("min-w-0 flex-1 truncate", option.labelClassName)}>
                      {option.label}
                    </span>
                  </QuietDropdownItem>
                );
              })}
            </QuietDropdownGroup>
          </QuietDropdownOptions>
        ) : (
          <QuietDropdownOptions key="fields" searchPlaceholder="Filter by...">
            <QuietDropdownEmpty>No filters.</QuietDropdownEmpty>
            <QuietDropdownGroup>
              {available.map((definition) => (
                <QuietDropdownItem
                  key={definition.key}
                  value={definition.key}
                  keywords={[definition.label]}
                  onSelect={() => {
                    onAdd(definition.key);
                    // Number filters are completed in their pill, not a value list.
                    if (definition.kind === 'number') {
                      setOpen(false);
                      return;
                    }
                    setSelectedKey(definition.key);
                  }}
                >
                  {definition.label}
                </QuietDropdownItem>
              ))}
            </QuietDropdownGroup>
          </QuietDropdownOptions>
        )}
      </PopoverContent>
    </Popover>
  );
}

export function PMFilterBar<K extends string>({
  definitions,
  values,
  visibleKeys,
  onToggle,
  onRemove,
  onClearAll,
  trailing,
  conditions,
  onConditionChange,
  optionModes,
  onOptionModeChange,
}: {
  definitions: PMFilterDefinition<K>[];
  values: PMFilterValues<K>;
  visibleKeys: Set<K>;
  onToggle: (key: K, value: string) => void;
  onRemove: (key: K) => void;
  onClearAll: () => void;
  trailing?: ReactNode;
  /** Number filter state, for definitions with kind "number". */
  conditions?: PMFilterConditions<K>;
  onConditionChange?: (key: K, condition: PMFilterCondition) => void;
  /** Chosen operator per options filter, for definitions with optionModes. */
  optionModes?: PMFilterOptionModes<K>;
  onOptionModeChange?: (key: K, mode: PMFilterOptionMode) => void;
}) {
  if (visibleKeys.size === 0) return null;

  return (
    <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-1.5 px-3 py-1.5">
      {definitions
        .filter((definition) => visibleKeys.has(definition.key))
        .map((definition) => definition.kind === 'number' ? (
          <PMConditionPill
            key={definition.key}
            definition={definition}
            condition={conditions?.[definition.key] ?? { operator: 'gt' }}
            onChange={(condition) => onConditionChange?.(definition.key, condition)}
            onRemove={() => onRemove(definition.key)}
          />
        ) : (
          <PMFilterPill
            key={definition.key}
            definition={definition}
            selected={values[definition.key] ?? []}
            onToggle={(value) => onToggle(definition.key, value)}
            onRemove={() => onRemove(definition.key)}
            mode={optionModes?.[definition.key]}
            onModeChange={onOptionModeChange ? (mode) => onOptionModeChange(definition.key, mode) : undefined}
          />
        ))}
      <Button
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-[10px] text-muted-foreground"
        onClick={onClearAll}
      >
        Clear all
      </Button>
      {trailing}
    </div>
  );
}

function PMOperatorSelect<V extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { value: V; label: string; name: string }[];
  value: V;
  onChange: (value: V) => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = options.find((option) => option.value === value) ?? options[0];
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={`${label} comparison: ${selected.name}`}
          className="rounded px-1 py-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        >
          {selected.label}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-44 p-0" align="start">
        <QuietDropdownOptions searchMode="off">
          <QuietDropdownGroup>
            {options.map((option) => (
              <QuietDropdownItem
                key={option.value}
                value={option.value}
                onSelect={() => {
                  onChange(option.value);
                  setOpen(false);
                }}
              >
                {option.label !== option.name ? (
                  <span className="w-14 shrink-0 text-muted-foreground">{option.label}</span>
                ) : null}
                <span className="min-w-0 flex-1 truncate">{option.name}</span>
                {option.value === value ? <Tick01Icon className="h-3 w-3" /> : null}
              </QuietDropdownItem>
            ))}
          </QuietDropdownGroup>
        </QuietDropdownOptions>
      </PopoverContent>
    </Popover>
  );
}

/** Number input that commits on blur or Enter, so typing does not refetch per keystroke. */
function PMConditionNumberInput({
  label,
  value,
  onCommit,
}: {
  label: string;
  value: string;
  onCommit: (value: string) => void;
}) {
  const [draft, setDraft] = useState(value);
  const [committed, setCommitted] = useState(value);
  if (value !== committed) {
    setCommitted(value);
    setDraft(value);
  }
  const commit = () => {
    if (draft.trim() !== value) onCommit(draft.trim());
  };
  return (
    <input
      type="number"
      inputMode="decimal"
      aria-label={label}
      placeholder="Number"
      value={draft}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'Enter') commit();
      }}
      className="w-20 rounded border border-border bg-background px-1.5 py-0.5 text-xs outline-none transition-colors [appearance:textfield] focus:border-foreground/40 [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
    />
  );
}

/** Applied number filter: field, comparison, number(s), remove. */
export function PMConditionPill<K extends string>({
  definition,
  condition,
  onChange,
  onRemove,
}: {
  definition: PMFilterDefinition<K>;
  condition: PMFilterCondition;
  onChange: (condition: PMFilterCondition) => void;
  onRemove: () => void;
}) {
  const { operator } = condition;
  const usesNoValue = operator === 'is_empty' || operator === 'is_not_empty';
  const changeOperator = (next: PMFilterNumberOperator) => {
    if (next === 'between') {
      const start = condition.value ?? condition.values?.[0] ?? '';
      onChange({ operator: next, values: [start, condition.values?.[1] ?? ''] });
    } else if (next === 'is_empty' || next === 'is_not_empty') {
      onChange({ operator: next });
    } else {
      onChange({ operator: next, value: condition.value ?? condition.values?.[0] ?? '' });
    }
  };
  return (
    <div className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">{definition.label}</span>
      <PMOperatorSelect
        label={definition.label}
        options={PM_FILTER_NUMBER_OPERATORS.map((option) => ({ value: option, label: NUMBER_OPERATOR_LABELS[option], name: NUMBER_OPERATOR_NAMES[option] }))}
        value={operator}
        onChange={changeOperator}
      />
      {operator === 'between' ? (
        <>
          <PMConditionNumberInput
            label={`${definition.label} minimum`}
            value={condition.values?.[0] ?? ''}
            onCommit={(start) => onChange({ operator, values: [start, condition.values?.[1] ?? ''] })}
          />
          <span className="text-muted-foreground/60">and</span>
          <PMConditionNumberInput
            label={`${definition.label} maximum`}
            value={condition.values?.[1] ?? ''}
            onCommit={(end) => onChange({ operator, values: [condition.values?.[0] ?? '', end] })}
          />
        </>
      ) : usesNoValue ? null : (
        <PMConditionNumberInput
          label={`${definition.label} value`}
          value={condition.value ?? ''}
          onCommit={(value) => onChange({ operator, value })}
        />
      )}
      {definition.note ? <span className="text-muted-foreground/60">· {definition.note}</span> : null}
      <button
        type="button"
        onClick={onRemove}
        className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
        aria-label={`Remove ${definition.label} filter`}
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </div>
  );
}
