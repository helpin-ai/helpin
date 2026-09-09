import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { gitService } from '@/lib/services/gitService';
import type { GitBranch } from '@/lib/pmTypes';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

const EMPTY_VALUE = '__empty__';
const CUSTOM_VALUE = '__custom__';

export function RepositoryBranchPicker({
  workspaceId,
  repositoryId,
  value,
  onChange,
  placeholder,
  emptyLabel,
  extraOptions = [],
  customLabel = 'Custom branch…',
  disabled = false,
  variant = 'form',
  width,
  triggerLabel,
}: {
  workspaceId: string;
  repositoryId?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  emptyLabel: string;
  extraOptions?: { value: string; label: string }[];
  customLabel?: string;
  disabled?: boolean;
  variant?: 'form' | 'sidebar';
  width?: string;
  triggerLabel?: ReactNode;
}) {
  const [branches, setBranches] = useState<GitBranch[]>([]);
  const [loading, setLoading] = useState(false);
  const [customMode, setCustomMode] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setBranches([]);
    setLoading(false);
    setCustomMode(false);
    setError('');

    if (!workspaceId || !repositoryId) {
      return;
    }

    const load = async () => {
      setLoading(true);
      const res = await gitService.listRepositoryBranches(workspaceId, repositoryId);
      if (cancelled) return;
      setLoading(false);
      if (res.error) {
        setError(res.error);
        return;
      }
      setBranches((res.data ?? []).slice().sort((a, b) => {
        if (a.is_default && !b.is_default) return -1;
        if (!a.is_default && b.is_default) return 1;
        return a.name.localeCompare(b.name);
      }));
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [repositoryId, workspaceId]);

  const valueOption = useMemo(() => {
    const trimmedValue = value.trim();
    if (!trimmedValue) {
      return null;
    }
    if (branches.some((branch) => branch.name === trimmedValue)) {
      return null;
    }
    if (extraOptions.some((option) => option.value === trimmedValue)) {
      return null;
    }
    return { value: trimmedValue, label: `Current selection (${trimmedValue})` };
  }, [branches, extraOptions, value]);

  const allOptions = useMemo(
    () => [...extraOptions, ...(valueOption ? [valueOption] : [])],
    [extraOptions, valueOption],
  );

  const branchNames = useMemo(
    () => new Set([...branches.map((branch) => branch.name), ...allOptions.map((option) => option.value)]),
    [branches, allOptions],
  );
  const selectOptions = useMemo(
    () => [
      { value: EMPTY_VALUE, label: emptyLabel },
      ...allOptions,
      ...branches.map((branch) => ({
        value: branch.name,
        label: branch.is_default ? `${branch.name} (default)` : branch.name,
      })),
      { value: CUSTOM_VALUE, label: customLabel },
    ],
    [allOptions, branches, customLabel, emptyLabel],
  );

  useEffect(() => {
    if (!value) {
      setCustomMode(false);
      return;
    }
    if (branchNames.has(value)) {
      setCustomMode(false);
    }
  }, [branchNames, value]);

  if (!repositoryId) {
    return (
      <div className="space-y-2">
        <div className={cn(
          'text-muted-foreground',
          variant === 'sidebar'
            ? 'flex min-h-6 items-center rounded-md px-1.5 py-0.5 text-xs'
            : 'flex h-9 items-center rounded-md border border-input bg-muted/40 px-3 text-sm',
        )}>
          Choose repository first
        </div>
      </div>
    );
  }

  const selectValue = customMode
    ? CUSTOM_VALUE
    : value
      ? (branchNames.has(value) ? value : CUSTOM_VALUE)
      : EMPTY_VALUE;

  const selectedOption = selectOptions.find((option) => option.value === selectValue);

  return (
    <div className="space-y-2">
      <SidebarPopoverSelect
        value={selectValue}
        options={selectOptions}
        onChange={(nextValue) => {
          if (nextValue === EMPTY_VALUE) {
            setCustomMode(false);
            onChange('');
            return;
          }
          if (nextValue === CUSTOM_VALUE) {
            setCustomMode(true);
            return;
          }
          setCustomMode(false);
          onChange(nextValue);
        }}
        width={width ?? (variant === 'sidebar' ? 'w-72' : 'w-80')}
        searchPlaceholder="Search branches..."
        disabled={disabled}
        showChevron={variant === 'form'}
        triggerClassName={
          variant === 'sidebar'
            ? 'flex w-full min-w-0 max-w-none items-center gap-1.5 rounded-md px-1.5 py-0.5 text-left text-xs hover:bg-accent'
            : 'flex h-9 w-full min-w-0 max-w-none items-center rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs hover:bg-accent/50'
        }
        renderTrigger={() => (
          triggerLabel ?? (
            <span className={cn(
              'block min-w-0 truncate',
              variant === 'sidebar' ? 'font-mono text-xs' : 'text-sm',
              selectValue === EMPTY_VALUE && 'text-muted-foreground',
            )}>
              {loading ? 'Loading branches…' : (selectedOption?.label || placeholder || emptyLabel)}
            </span>
          )
        )}
        renderOption={(optionValue) => {
          const option = selectOptions.find((entry) => entry.value === optionValue);
          return (
            <span className={cn(
              'truncate',
              optionValue === EMPTY_VALUE && 'text-muted-foreground',
              optionValue !== EMPTY_VALUE && optionValue !== CUSTOM_VALUE && optionValue !== valueOption?.value && 'font-mono',
            )}>
              {option?.label ?? optionValue}
            </span>
          );
        }}
      />
      {error && (
        <p className="text-xs text-muted-foreground">
          Could not load branches. You can still enter one manually.
        </p>
      )}
      {customMode && (
        <Input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder}
          disabled={disabled}
        />
      )}
    </div>
  );
}
