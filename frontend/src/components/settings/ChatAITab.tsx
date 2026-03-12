import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { useChatSettings, useUpdateChatSettings } from '@/hooks/queries';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import type { BusinessHoursDay } from '@/lib/pmTypes';

const DAYS = [
  { key: 'mon', label: 'Monday' },
  { key: 'tue', label: 'Tuesday' },
  { key: 'wed', label: 'Wednesday' },
  { key: 'thu', label: 'Thursday' },
  { key: 'fri', label: 'Friday' },
  { key: 'sat', label: 'Saturday' },
  { key: 'sun', label: 'Sunday' },
];

const COMMON_TIMEZONES = [
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Anchorage',
  'Pacific/Honolulu',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Kolkata',
  'Australia/Sydney',
  'Pacific/Auckland',
  'UTC',
];

export function ChatAITab({ workspaceId }: { workspaceId: string }) {
  const { data, isLoading } = useChatSettings(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);

  const [aiEnabled, setAiEnabled] = useState(false);
  const [confidenceThreshold, setConfidenceThreshold] = useState('0.7');
  const [showTalkToHuman, setShowTalkToHuman] = useState(true);
  const [handoffBehavior, setHandoffBehavior] = useState('unassigned');
  const [handoffTeamId, setHandoffTeamId] = useState<string | null>(null);
  const [businessHoursEnabled, setBusinessHoursEnabled] = useState(false);
  const [timezone, setTimezone] = useState('America/New_York');
  const [schedule, setSchedule] = useState<Record<string, BusinessHoursDay>>({});
  const [outsideMessage, setOutsideMessage] = useState('');
  const [csatEnabled, setCsatEnabled] = useState(false);

  useEffect(() => {
    if (data?.settings) {
      const s = data.settings;
      setAiEnabled(s.ai_enabled);
      setConfidenceThreshold(String(s.ai_confidence_threshold));
      setShowTalkToHuman(s.show_talk_to_human);
      setHandoffBehavior(s.handoff_behavior);
      setHandoffTeamId(s.handoff_team_id);
      setBusinessHoursEnabled(s.business_hours_enabled);
      setTimezone(s.business_hours_timezone);
      setSchedule(s.business_hours_schedule);
      setOutsideMessage(s.outside_hours_message);
      setCsatEnabled(s.csat_enabled);
    }
  }, [data]);

  const handleSave = () => {
    updateMutation.mutate({
      ai_enabled: aiEnabled,
      ai_confidence_threshold: parseFloat(confidenceThreshold),
      show_talk_to_human: showTalkToHuman,
      handoff_behavior: handoffBehavior,
      handoff_team_id: handoffBehavior === 'assign_to_team' ? handoffTeamId : null,
      business_hours_enabled: businessHoursEnabled,
      business_hours_timezone: timezone,
      business_hours_schedule: schedule,
      outside_hours_message: outsideMessage,
      csat_enabled: csatEnabled,
    }, {
      onSuccess: () => toast.success('Settings saved'),
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to save'),
    });
  };

  const updateDay = (dayKey: string, patch: Partial<BusinessHoursDay>) => {
    setSchedule(prev => ({
      ...prev,
      [dayKey]: { ...prev[dayKey], ...patch },
    }));
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div />
        <Button onClick={handleSave} disabled={updateMutation.isPending} size="sm">
          {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      {/* AI Auto-Reply */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">AI Auto-Reply</CardTitle>
          <CardDescription>Configure AI-powered automatic responses to customer messages.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Enable AI auto-reply</Label>
              <p className="text-xs text-muted-foreground">AI will attempt to answer questions using your knowledge base.</p>
            </div>
            <Switch checked={aiEnabled} onCheckedChange={setAiEnabled} />
          </div>

          <div className="space-y-2">
            <Label className="text-sm">Confidence Threshold</Label>
            <Select value={confidenceThreshold} onValueChange={setConfidenceThreshold}>
              <SelectTrigger className="w-32">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0].map(v => (
                  <SelectItem key={v} value={String(v)}>{(v * 100).toFixed(0)}%</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">AI will only respond when confidence is at or above this level.</p>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Show "Talk to Human" button</Label>
              <p className="text-xs text-muted-foreground">Let visitors escalate to a human agent at any time.</p>
            </div>
            <Switch checked={showTalkToHuman} onCheckedChange={setShowTalkToHuman} />
          </div>
        </CardContent>
      </Card>

      {/* Handoff Routing */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Handoff Routing</CardTitle>
          <CardDescription>How conversations are assigned when a human agent is needed.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-sm">Handoff Behavior</Label>
            <Select value={handoffBehavior} onValueChange={setHandoffBehavior}>
              <SelectTrigger className="w-56">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unassigned">Leave unassigned</SelectItem>
                <SelectItem value="assign_to_team">Assign to team</SelectItem>
                <SelectItem value="round_robin">Round robin</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {handoffBehavior === 'assign_to_team' && (
            <div className="space-y-2">
              <Label className="text-sm">Team</Label>
              <Select value={handoffTeamId ?? ''} onValueChange={setHandoffTeamId}>
                <SelectTrigger className="w-56">
                  <SelectValue placeholder="Select a team..." />
                </SelectTrigger>
                <SelectContent>
                  {teams.map(team => (
                    <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Business Hours */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Business Hours</CardTitle>
          <CardDescription>Set when your team is available. Outside these hours, customers see an offline message.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Enable business hours</Label>
              <p className="text-xs text-muted-foreground">Widget shows online/offline status based on schedule.</p>
            </div>
            <Switch checked={businessHoursEnabled} onCheckedChange={setBusinessHoursEnabled} />
          </div>

          {businessHoursEnabled && (
            <>
              <div className="space-y-2">
                <Label className="text-sm">Timezone</Label>
                <Select value={timezone} onValueChange={setTimezone}>
                  <SelectTrigger className="w-64">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {COMMON_TIMEZONES.map(tz => (
                      <SelectItem key={tz} value={tz}>{tz.replace(/_/g, ' ')}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label className="text-sm">Schedule</Label>
                <div className="space-y-1.5">
                  {DAYS.map(({ key, label }) => {
                    const day = schedule[key] ?? { start: '09:00', end: '17:00', enabled: false };
                    return (
                      <div key={key} className="flex items-center gap-3">
                        <div className="w-24">
                          <Switch
                            checked={day.enabled}
                            onCheckedChange={(v) => updateDay(key, { enabled: v })}
                          />
                          <span className="ml-2 text-sm">{label.slice(0, 3)}</span>
                        </div>
                        <Input
                          type="time"
                          value={day.start}
                          onChange={(e) => updateDay(key, { start: e.target.value })}
                          disabled={!day.enabled}
                          className="w-28 h-8 text-sm"
                        />
                        <span className="text-xs text-muted-foreground">to</span>
                        <Input
                          type="time"
                          value={day.end}
                          onChange={(e) => updateDay(key, { end: e.target.value })}
                          disabled={!day.enabled}
                          className="w-28 h-8 text-sm"
                        />
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="outside-msg" className="text-sm">Outside Hours Message</Label>
                <Textarea
                  id="outside-msg"
                  value={outsideMessage}
                  onChange={(e) => setOutsideMessage(e.target.value)}
                  placeholder="We're currently offline. Leave a message and we'll get back to you!"
                  rows={2}
                />
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* CSAT */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Customer Satisfaction</CardTitle>
          <CardDescription>Collect feedback after conversations are resolved.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Enable CSAT surveys</Label>
              <p className="text-xs text-muted-foreground">Send a satisfaction survey after conversation resolution.</p>
            </div>
            <Switch checked={csatEnabled} onCheckedChange={setCsatEnabled} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
