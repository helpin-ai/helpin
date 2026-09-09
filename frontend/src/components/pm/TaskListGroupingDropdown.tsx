import { QuietFilterDropdown } from '@/components/design-system/quiet';

interface TaskListGroupingDropdownProps<T extends string> {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}

/** Use the same Group by control as the epics list in both task-list toolbars. */
export function TaskListGroupingDropdown<T extends string>({
  value,
  options,
  onChange,
}: TaskListGroupingDropdownProps<T>) {
  return (
    <QuietFilterDropdown
      label="Group by"
      showLabel="inline"
      value={value}
      options={options}
      onChange={next => onChange(next as T)}
    />
  );
}
