import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { DialogFooter } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { SCALE_DESCRIPTIONS, SCALE_LABELS, getEstimateOptions } from '@/lib/estimateScales';
import type { EstimateScale, TeamEstimateSettings } from '@/lib/types';

export function EstimateSettingsForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamEstimateSettings | null;
  saving: boolean;
  onSave: (data: { enabled?: boolean; scale?: EstimateScale; extended?: boolean; allow_zero?: boolean; count_unestimated_as_one?: boolean }) => void;
}) {
  const [enabled, setEnabled] = useState(initial?.enabled ?? false);
  const [scale, setScale] = useState<EstimateScale>(initial?.scale ?? 'linear');
  const [extended, setExtended] = useState(initial?.extended ?? false);
  const [allowZero, setAllowZero] = useState(initial?.allow_zero ?? false);
  const [countUnestimated, setCountUnestimated] = useState(initial?.count_unestimated_as_one ?? true);

  useEffect(() => {
    setEnabled(initial?.enabled ?? false);
    setScale(initial?.scale ?? 'linear');
    setExtended(initial?.extended ?? false);
    setAllowZero(initial?.allow_zero ?? false);
    setCountUnestimated(initial?.count_unestimated_as_one ?? true);
  }, [initial, teamId]);

  const scaleOptions = getEstimateOptions(scale, extended, allowZero);

  return (
    <div className="space-y-5 py-2">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Enable estimates</p>
          <p className="text-xs text-muted-foreground">Show effort estimates on stories</p>
        </div>
        <Switch checked={enabled} onCheckedChange={setEnabled} />
      </div>

      {enabled && (
        <>
          <div className="space-y-2">
            <Label className="text-sm">Scale</Label>
            <Select value={scale} onValueChange={(value) => setScale(value as EstimateScale)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(SCALE_LABELS) as EstimateScale[]).map((value) => (
                  <SelectItem key={value} value={value}>
                    <span className="font-medium">{SCALE_LABELS[value]}</span>
                    <span className="ml-2 text-muted-foreground">{SCALE_DESCRIPTIONS[value]}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="rounded-md border border-border bg-muted/40 px-3 py-2">
            <p className="mb-1 text-xs text-muted-foreground">Scale values</p>
            <div className="flex flex-wrap gap-1.5">
              {scaleOptions.map((option) => (
                <span key={option.value} className="inline-flex items-center rounded-md border border-border bg-background px-2 py-0.5 text-xs font-medium">
                  {option.label}
                </span>
              ))}
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Extended scale</p>
              <p className="text-xs text-muted-foreground">Add two additional larger values</p>
            </div>
            <Switch checked={extended} onCheckedChange={setExtended} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Allow zero estimates</p>
              <p className="text-xs text-muted-foreground">Allow stories to be estimated as zero effort</p>
            </div>
            <Switch checked={allowZero} onCheckedChange={setAllowZero} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Count unestimated as 1 point</p>
              <p className="text-xs text-muted-foreground">Unestimated stories count as 1 point in calculations</p>
            </div>
            <Switch checked={countUnestimated} onCheckedChange={setCountUnestimated} />
          </div>
        </>
      )}

      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave({ enabled, scale, extended, allow_zero: allowZero, count_unestimated_as_one: countUnestimated })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}
