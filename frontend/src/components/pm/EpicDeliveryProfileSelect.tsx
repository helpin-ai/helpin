import { modelNameWithThinking } from "@/lib/aiProviders";
import { useAIProfiles } from '@/hooks/queries/useAIProfiles';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';

export function EpicDeliveryProfileSelect({ workspaceId, value, onChange, disabled = false }: {
  workspaceId: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const profiles = useAIProfiles(workspaceId);
  const choices = (profiles.data ?? []).filter(profile => !profile.hidden_from_ask_agent || profile.id === value);
  return (
    <div className="space-y-2">
      <label className="text-xs font-medium text-foreground">AI model for all agent steps</label>
      <Select value={value || '__defaults__'} onValueChange={(next) => onChange(next === '__defaults__' ? '' : next)} disabled={disabled || profiles.isPending}>
        <SelectTrigger variant="underline" className="w-full" aria-label="AI model for delivery agents"><SelectValue placeholder="Agent defaults" /></SelectTrigger>
        <SelectContent>
          <SelectItem value="__defaults__">Agent defaults</SelectItem>
          <SelectGroup>
            <SelectLabel>Workspace-wide models</SelectLabel>
            {choices.filter((profile) => profile.scope === 'workspace').map((profile) => <SelectItem key={profile.id} value={profile.id}>{modelNameWithThinking(profile.name, profile.primary.model.controls.reasoning_effort)}</SelectItem>)}
          </SelectGroup>
          <SelectGroup>
            <SelectLabel>Personal models</SelectLabel>
            {choices.filter((profile) => profile.scope === 'personal').map((profile) => <SelectItem key={profile.id} value={profile.id}>{modelNameWithThinking(profile.name, profile.primary.model.controls.reasoning_effort)}</SelectItem>)}
          </SelectGroup>
        </SelectContent>
      </Select>
      {profiles.isError ? <p className="text-xs text-destructive">Models could not be loaded. Agent defaults remain available.</p> : null}
      <p className="text-xs text-muted-foreground">Your model choice is saved for future steps. Personal models remain tied to your account.</p>
    </div>
  );
}
