import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';

import type { AgentRunDeliveryMode } from '@/lib/pmTypes';

export function AgentRunDeliveryModePicker({ value, onChange }: {
  value: AgentRunDeliveryMode;
  onChange: (value: AgentRunDeliveryMode) => void;
}) {
  return (
    <div className="space-y-1">
      <Select value={value} onValueChange={(next) => onChange(next as AgentRunDeliveryMode)}>
        <SelectTrigger aria-label="Repository delivery"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="publish">Publish changes</SelectItem>
          <SelectItem value="preview">Preview changes</SelectItem>
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        {value === 'preview' ? 'Keep repository changes local. No automatic push or pull request.' : 'Push repository changes and open a pull request after success.'}
      </p>
    </div>
  );
}
