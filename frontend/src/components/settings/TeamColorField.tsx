import { ColorPicker } from '@/components/pm/ColorPicker';
import { TeamBadge } from '@/components/workspace/TeamBadge';
import { Button } from '@/components/ui/button';
import { TEAM_PRESET_COLORS } from '@/lib/teamColor';

export function TeamColorField({ name, value, onChange, showDefault = true }: {
  name: string;
  value: string;
  onChange: (color: string) => void;
  showDefault?: boolean;
}) {
  return (
    <fieldset className="min-w-0 space-y-2">
      <legend className="float-left mb-2 flex min-h-6 w-full items-center justify-between text-sm font-medium">
        Color
        {showDefault && (
          <Button type="button" variant={value ? 'ghost' : 'secondary'} size="sm" className="h-6 px-2 text-xs" aria-pressed={!value} onClick={() => onChange('')}>
            Default
          </Button>
        )}
      </legend>
      <div className="clear-both flex items-center gap-3">
        <TeamBadge name={name || 'Team'} color={value} />
        <ColorPicker value={value} onChange={onChange} shape="square" presets={TEAM_PRESET_COLORS} />
      </div>
    </fieldset>
  );
}
