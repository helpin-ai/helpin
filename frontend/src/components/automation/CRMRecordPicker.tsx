import { useEffect, useId, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Command, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { QuietTextAction, quietUnderlineControlClassName } from '@/components/design-system/quiet';
import { CRM_RECORD_TARGETS, type CRMRecordTargetType } from '@/lib/agentCRMTargets';
import { getCRMAgentRecord, listCRMAgentRecords } from '@/lib/services/crmAgentRecordService';
import { cn } from '@/lib/utils';
import { queryKeys } from '@/lib/queryKeys';

interface CRMRecordPickerProps {
  workspaceId: string;
  targetType: CRMRecordTargetType;
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
  id?: string;
}

export function CRMRecordPicker(props: CRMRecordPickerProps) {
  // Scope UI state as well as cached data to the workspace and record type.
  return <ScopedCRMRecordPicker key={`${props.workspaceId}:${props.targetType}`} {...props} />;
}

function ScopedCRMRecordPicker({ workspaceId, targetType, value, onChange, disabled, id }: CRMRecordPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState('');
  const queryClient = useQueryClient();
  const listId = useId();
  const labels = CRM_RECORD_TARGETS[targetType];
  const singular = labels.singular.toLowerCase();
  const selectedKey = queryKeys.crm.automationRecord(workspaceId, targetType, value);
  useEffect(() => {
    const timer = window.setTimeout(() => setQuery(search.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [search]);

  const records = useQuery({
    queryKey: queryKeys.crm.automationRecords(workspaceId, targetType, query),
    queryFn: () => listCRMAgentRecords(workspaceId, targetType, query),
    enabled: open && !!workspaceId && !disabled,
    retry: false,
  });
  const selected = useQuery({
    queryKey: selectedKey,
    queryFn: () => getCRMAgentRecord(workspaceId, targetType, value),
    enabled: !!workspaceId && !!value,
    retry: false,
  });
  const searching = search.trim() !== query || records.isFetching;
  const selectionLabel = !value ? `Choose a ${singular}`
    : selected.isError ? `${labels.singular} unavailable`
      : selected.data?.name || `Loading ${singular}…`;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          id={id}
          type="button"
          role="combobox"
          aria-label={`Choose a CRM ${singular}`}
          aria-expanded={open}
          aria-controls={open ? listId : undefined}
          disabled={disabled || !workspaceId}
          className={cn(quietUnderlineControlClassName, 'min-w-48 max-w-full truncate text-left disabled:cursor-not-allowed disabled:opacity-50')}
          title={selectionLabel}
        >
          {selectionLabel}
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 max-w-[calc(100vw-2rem)] p-0">
        <Command shouldFilter={false}>
          <CommandInput value={search} onValueChange={setSearch} placeholder={`Search ${labels.plural.toLowerCase()}…`} />
          <CommandList id={listId} aria-busy={searching}>
            {searching ? (
              <p role="status" className="px-3 py-4 text-sm text-quiet-text-secondary">Searching {labels.plural.toLowerCase()}…</p>
            ) : records.isError ? (
              <div role="alert" className="px-3 py-4 text-sm text-quiet-text-secondary">
                <p>Couldn’t load {labels.plural.toLowerCase()}. Check your CRM access or try again.</p>
                <QuietTextAction onClick={() => void records.refetch()}>Try again</QuietTextAction>
              </div>
            ) : records.data?.items.length === 0 ? (
              <p role="status" className="px-3 py-4 text-sm text-quiet-text-secondary">
                {query ? `No matching ${labels.plural.toLowerCase()}. Try another search.` : `No ${labels.plural.toLowerCase()} available.`}
              </p>
            ) : records.data?.items.map((record) => (
              <CommandItem
                key={record.id}
                value={record.id}
                onSelect={() => {
                  queryClient.setQueryData(queryKeys.crm.automationRecord(workspaceId, targetType, record.id), record);
                  onChange(record.id);
                  setOpen(false);
                }}
                className="min-w-0"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{record.name}</span>
                  {record.detail ? <span className="block truncate text-xs text-quiet-text-secondary">{record.detail}</span> : null}
                </span>
                {record.id === value ? <span className="text-xs text-quiet-text-secondary">Selected</span> : null}
              </CommandItem>
            ))}
          </CommandList>
          {!searching && !records.isError && records.data && records.data.total > records.data.items.length ? (
            <p className="border-t border-quiet-divider-strong px-3 py-2 text-xs text-quiet-text-secondary">Showing {records.data.items.length} of {records.data.total}. Refine your search to find another {singular}.</p>
          ) : null}
        </Command>
      </PopoverContent>
    </Popover>
  );
}
