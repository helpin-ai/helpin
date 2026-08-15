import { useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import { detectMeetingPlatform } from '@/lib/meetingPresentation';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useCreateCRMMeeting, useCRMMeetingSettings } from '@/hooks/queries/useCRMMeetings';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';

export function CreateMeetingDialog({
  open,
  onOpenChange,
  workspaceId,
  workspaceSlug,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workspaceSlug: string;
}) {
  const navigate = useNavigate();
  const createMeeting = useCreateCRMMeeting(workspaceId);
  const { data: settingsData } = useCRMMeetingSettings(workspaceId);
  const [title, setTitle] = useState('');
  const [meetingUrl, setMeetingUrl] = useState('');
  const [startNow, setStartNow] = useState(true);
  const [scheduledStart, setScheduledStart] = useState('');
  const [recordAudioOverride, setRecordAudioOverride] = useState<boolean | null>(null);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const idempotencyKey = useRef(crypto.randomUUID());
  const recordAudio = recordAudioOverride ?? settingsData?.settings.record_audio_by_default ?? false;
  const detectedPlatform = detectMeetingPlatform(meetingUrl);
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setTitle('');
      setMeetingUrl('');
      setStartNow(true);
      setScheduledStart('');
      setRecordAudioOverride(null);
      idempotencyKey.current = crypto.randomUUID();
    }
    onOpenChange(nextOpen);
  };

  const submit = async () => {
    try {
      const detail = await createMeeting.mutateAsync({
        payload: {
          workspace_id: workspaceId,
          title: title.trim(),
          meeting_url: meetingUrl.trim(),
          scheduled_start_at: !startNow && scheduledStart ? new Date(scheduledStart).toISOString() : undefined,
          record_audio: recordAudio,
          start_now: startNow,
        },
        idempotencyKey: idempotencyKey.current,
      });
      handleOpenChange(false);
      toast.success(startNow ? 'Helpin is joining the meeting' : 'Meeting scheduled');
      void navigate({
        to: '/w/$slug/crm/meetings/$meetingId',
        params: { slug: workspaceSlug, meetingId: detail.meeting.id },
      });
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) {
        setUpgradeReason(reason);
        return;
      }
      toast.error(error instanceof Error ? error.message : 'Unable to create meeting');
    }
  };

  const settingsEnabled = settingsData?.settings.enabled ?? false;

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Add meeting</DialogTitle>
          <DialogDescription>Paste a supported Google Meet, Zoom, Teams, or Webex URL.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {!settingsEnabled && (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 text-sm text-amber-700 dark:text-amber-300">
              Meeting notes are turned off. An admin must enable them before the notetaker can join.
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="meeting-title">Title</Label>
            <Input id="meeting-title" value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Discovery call with Acme" autoFocus />
          </div>
          <div className="space-y-2">
            <Label htmlFor="meeting-url">Meeting URL</Label>
            <Input id="meeting-url" type="url" value={meetingUrl} onChange={(event) => setMeetingUrl(event.target.value)} placeholder="https://meet.google.com/abc-defg-hij" />
            {detectedPlatform && <div className="flex items-center gap-2 text-xs text-muted-foreground"><MeetingPlatformLabel platform={detectedPlatform} compact /><span>link detected</span></div>}
          </div>
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div>
              <Label>Join now</Label>
              <p className="text-xs text-muted-foreground">Start a capture attempt as soon as the meeting is created.</p>
            </div>
            <Switch checked={startNow} onCheckedChange={setStartNow} />
          </div>
          {!startNow && (
            <div className="space-y-2">
              <Label htmlFor="meeting-start">Scheduled start</Label>
              <Input id="meeting-start" type="datetime-local" value={scheduledStart} onChange={(event) => setScheduledStart(event.target.value)} />
              <p className="text-xs text-muted-foreground">Scheduled meetings stay in Helpin until someone starts the capture. Calendar auto-join is a later phase.</p>
            </div>
          )}
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div>
              <Label>Record audio</Label>
              <p className="mt-1 text-xs text-muted-foreground">Meetings are workspace-visible in this release.</p>
            </div>
            <Switch checked={recordAudio} onCheckedChange={setRecordAudioOverride} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>Cancel</Button>
          <Button onClick={submit} disabled={createMeeting.isPending || !title.trim() || !meetingUrl.trim() || (startNow && !settingsEnabled)}>
            {createMeeting.isPending ? 'Creating…' : startNow ? 'Create & join' : 'Schedule meeting'}
          </Button>
        </DialogFooter>
      </DialogContent>
      </Dialog>
      <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(dialogOpen) => { if (!dialogOpen) setUpgradeReason(null); }} reason={upgradeReason} />
    </>
  );
}
