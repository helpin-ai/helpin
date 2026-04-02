import { useEffect, useState } from 'react';
import { CircleHelp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DialogFooter } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Switch } from '@/components/ui/switch';
import type { PMAutomation } from '@/lib/pmTypes';

const DURATION_OPTIONS = [
  { value: '1', label: '1 week' },
  { value: '2', label: '2 weeks' },
  { value: '3', label: '3 weeks' },
  { value: '4', label: '4 weeks' },
];

const START_DAY_OPTIONS = [
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
  { value: '0', label: 'Sunday' },
];

const UPCOMING_COUNT_OPTIONS = [
  { value: '1', label: '1 sprint' },
  { value: '2', label: '2 sprints' },
  { value: '3', label: '3 sprints' },
  { value: '4', label: '4 sprints' },
  { value: '5', label: '5 sprints' },
];

interface SprintSettingsFormProps {
  teamId: string;
  sprintsEnabled: boolean;
  autoCreateConfig: PMAutomation | null;
  moveUnfinishedConfig: PMAutomation | null;
  saving: boolean;
  onSave: (data: {
    sprints_enabled: boolean;
    auto_create_enabled: boolean;
    auto_create_upcoming_count: number;
    auto_create_duration_weeks: number;
    auto_create_start_day: number;
    move_unfinished_enabled: boolean;
  }) => void;
}

export function SprintSettingsForm({
  teamId,
  sprintsEnabled,
  autoCreateConfig,
  moveUnfinishedConfig,
  saving,
  onSave,
}: SprintSettingsFormProps) {
  const [enabled, setEnabled] = useState(sprintsEnabled);
  const [autoCreate, setAutoCreate] = useState(autoCreateConfig?.enabled ?? false);
  const [durationWeeks, setDurationWeeks] = useState(String(autoCreateConfig?.config_int2 ?? 2));
  const [startDay, setStartDay] = useState(String(autoCreateConfig?.config_int3 ?? 1));
  const [upcomingCount, setUpcomingCount] = useState(String(autoCreateConfig?.config_int ?? 1));
  const [moveUnfinished, setMoveUnfinished] = useState(moveUnfinishedConfig?.enabled ?? false);

  useEffect(() => {
    setEnabled(sprintsEnabled);
    setAutoCreate(autoCreateConfig?.enabled ?? false);
    setDurationWeeks(String(autoCreateConfig?.config_int2 ?? 2));
    setStartDay(String(autoCreateConfig?.config_int3 ?? 1));
    setUpcomingCount(String(autoCreateConfig?.config_int ?? 1));
    setMoveUnfinished(moveUnfinishedConfig?.enabled ?? false);
  }, [teamId, sprintsEnabled, autoCreateConfig, moveUnfinishedConfig]);

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Sprints help create rhythm and focus for your team over short, time-boxed windows.
        Automations handle creating upcoming sprints, rolling over unfinished work, and tracking progress across cycles.
      </p>

      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Enable sprints</p>
          <p className="text-xs text-muted-foreground">Show sprints in sidebar and allow sprint planning</p>
        </div>
        <Switch checked={enabled} onCheckedChange={setEnabled} />
      </div>

      {enabled && (
        <>
          <div className="space-y-4">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Automation</p>

            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Roll over unfinished work</p>
                <p className="text-xs text-muted-foreground">When a sprint ends, move incomplete tasks to the next sprint</p>
              </div>
              <Switch checked={moveUnfinished} onCheckedChange={setMoveUnfinished} />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Auto-create sprints</p>
                <p className="text-xs text-muted-foreground">Automatically create new sprints to maintain a pipeline of upcoming cycles</p>
              </div>
              <Switch checked={autoCreate} onCheckedChange={setAutoCreate} />
            </div>

            {autoCreate && (
              <div className="ml-1 space-y-3 border-l-2 border-border/60 pl-4">
                <div className="flex items-center justify-between">
                  <Label className="text-sm">Each sprint lasts</Label>
                  <Select value={durationWeeks} onValueChange={setDurationWeeks}>
                    <SelectTrigger className="w-[140px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {DURATION_OPTIONS.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="flex items-center justify-between">
                  <Label className="text-sm">Sprints start on</Label>
                  <Select value={startDay} onValueChange={setStartDay}>
                    <SelectTrigger className="w-[140px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {START_DAY_OPTIONS.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-1.5 text-sm">
                    Upcoming sprints to create
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <CircleHelp className="h-3.5 w-3.5 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[220px] text-xs">
                        How many unstarted sprints to keep ready ahead of the active sprint.
                      </TooltipContent>
                    </Tooltip>
                  </Label>
                  <Select value={upcomingCount} onValueChange={setUpcomingCount}>
                    <SelectTrigger className="w-[140px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {UPCOMING_COUNT_OPTIONS.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}
          </div>
        </>
      )}

      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave({
            sprints_enabled: enabled,
            auto_create_enabled: autoCreate,
            auto_create_upcoming_count: parseInt(upcomingCount, 10),
            auto_create_duration_weeks: parseInt(durationWeeks, 10),
            auto_create_start_day: parseInt(startDay, 10),
            move_unfinished_enabled: moveUnfinished,
          })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}
