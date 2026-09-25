import { useAIProfiles } from '@/hooks/queries/useAIProfiles';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';

export function EpicDeliveryProfileSelect({ workspaceId, value, onChange, disabled = false }: {
  workspaceId: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const profiles = useAIProfiles(workspaceId);
  return (
    <div className="space-y-2">
      <label className="text-xs font-medium text-foreground">AI profile for all agent steps</label>
      <Select value={value || '__defaults__'} onValueChange={(next) => onChange(next === '__defaults__' ? '' : next)} disabled={disabled || profiles.isPending}>
        <SelectTrigger variant="underline" className="w-full" aria-label="AI profile for delivery agents"><SelectValue placeholder="Agent defaults" /></SelectTrigger>
        <SelectContent>
          <SelectItem value="__defaults__">Agent defaults</SelectItem>
          <SelectGroup>
            <SelectLabel>Shared profiles</SelectLabel>
            {(profiles.data ?? []).filter((profile) => profile.scope === 'workspace').map((profile) => <SelectItem key={profile.id} value={profile.id}>{profile.name}</SelectItem>)}
          </SelectGroup>
          <SelectGroup>
            <SelectLabel>My profiles</SelectLabel>
            {(profiles.data ?? []).filter((profile) => profile.scope === 'personal').map((profile) => <SelectItem key={profile.id} value={profile.id}>{profile.name}</SelectItem>)}
          </SelectGroup>
        </SelectContent>
      </Select>
      {profiles.isError ? <p className="text-xs text-destructive">Profiles could not be loaded. Agent defaults remain available.</p> : null}
      <p className="text-xs text-muted-foreground">An explicit profile is saved for future steps. Personal profiles remain tied to your account.</p>
    </div>
  );
}
