import { useId } from 'react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Label } from '@/components/ui/label';

import type { AgentRunDeliveryMode } from '@/lib/pmTypes';

export function AgentRunDeliveryModePicker({ value, onChange }: {
  value: AgentRunDeliveryMode;
  onChange: (value: AgentRunDeliveryMode) => void;
}) {
  const id = useId();
  return (
    <div className="min-w-0 space-y-2">
      <Label htmlFor={id}>Repository delivery</Label>
      <Select value={value} onValueChange={(next) => onChange(next as AgentRunDeliveryMode)}>
        <SelectTrigger id={id} aria-label="Repository delivery" className="w-full"><SelectValue /></SelectTrigger>
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
