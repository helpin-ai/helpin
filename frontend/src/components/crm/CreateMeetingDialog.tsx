import { useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import {
  QuietPrimaryAction,
  QuietStatusText,
  QuietTextAction,
  QuietUnderlineInput,
} from '@/components/design-system/quiet';
import { detectMeetingPlatform } from '@/lib/meetingPresentation';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useCreateCRMMeeting, useCRMMeetingSettings } from '@/hooks/queries/useCRMMeetings';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { associationsService } from '@/lib/services/associationsService';
import type { CRMObjectType } from '@/lib/crmTypes';

export function CreateMeetingDialog({
  open,
  onOpenChange,
  workspaceId,
  workspaceSlug,
  associationContext,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workspaceSlug: string;
  associationContext?: {
    type: Extract<CRMObjectType, 'contact' | 'company' | 'deal'>;
    id: string;
  };
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
  const recordAudio = recordAudioOverride ?? settingsData?.settings.record_audio_by_default ?? true;
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
      if (associationContext) {
        const association = await associationsService.createAssociation({
          workspace_id: workspaceId,
          from_object_type: 'meeting',
          from_object_id: detail.meeting.id,
          to_object_type: associationContext.type,
          to_object_id: associationContext.id,
        });
        if (association.error) throw new Error(association.error);
      }
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

  const settingsEnabled = settingsData?.settings.enabled ?? true;

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent variant="flush" className="sm:max-w-lg">
          <DialogHeader className="border-b border-quiet-divider-strong px-6 py-5 pr-14">
            <DialogTitle className="text-[20px] font-semibold leading-tight tracking-[-0.018em] text-quiet-text-primary">
              Add meeting
            </DialogTitle>
            <DialogDescription className="text-sm leading-[1.6] text-quiet-text-tertiary">
              Paste a Google Meet, Zoom, Teams, or Webex URL and choose when Helpin should join.
            </DialogDescription>
          </DialogHeader>
          <div>
            {!settingsEnabled && (
              <div className="border-b border-quiet-divider-strong px-6 py-4">
                <QuietStatusText tone="blocker" className="text-quiet-accent">Meeting notes are off</QuietStatusText>
                <p className="mt-1 text-sm leading-[1.6] text-quiet-text-tertiary">
                  A workspace admin must enable meeting notes before Helpin can join immediately.
                </p>
              </div>
            )}
            <div className="space-y-5 px-6 py-5">
              <div>
                <Label htmlFor="meeting-title" className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">
                  Meeting title
                </Label>
                <QuietUnderlineInput
                  id="meeting-title"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder="Discovery call with Acme"
                  className="mt-1.5 w-full"
                  autoFocus
                />
              </div>
              <div>
                <Label htmlFor="meeting-url" className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">
                  Meeting URL
                </Label>
                <QuietUnderlineInput
                  id="meeting-url"
                  type="url"
                  value={meetingUrl}
                  onChange={(event) => setMeetingUrl(event.target.value)}
                  placeholder="https://meet.google.com/abc-defg-hij"
                  className="mt-1.5 w-full"
                />
                {detectedPlatform ? (
                  <div className="mt-2 flex items-center gap-2 text-[11.5px] text-quiet-muted">
                    <MeetingPlatformLabel platform={detectedPlatform} compact presentation="quiet" />
                    <span aria-hidden="true" className="h-[10px] w-px bg-quiet-meta-separator" />
                    <QuietStatusText tone="positive" className="text-quiet-positive">Link detected</QuietStatusText>
                  </div>
                ) : null}
              </div>
            </div>
            <div className="border-y border-quiet-divider-strong">
              <div className="flex items-center justify-between gap-5 px-6 py-4">
                <div>
                  <Label htmlFor="meeting-join-now" className="text-sm font-medium text-quiet-text-primary">Join now</Label>
                  <p className="mt-0.5 text-[12.5px] leading-5 text-quiet-text-tertiary">
                    Start capturing as soon as this meeting is created.
                  </p>
                </div>
                <Switch id="meeting-join-now" checked={startNow} onCheckedChange={setStartNow} />
              </div>
              {!startNow ? (
                <div className="border-t border-quiet-divider-light px-6 py-4">
                  <Label htmlFor="meeting-start" className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">
                    Scheduled start
                  </Label>
                  <QuietUnderlineInput
                    id="meeting-start"
                    type="datetime-local"
                    value={scheduledStart}
                    onChange={(event) => setScheduledStart(event.target.value)}
                    className="mt-1.5 w-full"
                  />
                  <p className="mt-2 text-[12.5px] leading-5 text-quiet-text-tertiary">
                    Calendar meetings can also be selected for automatic joining from the Meetings page.
                  </p>
                </div>
              ) : null}
              <div className="flex items-center justify-between gap-5 border-t border-quiet-divider-light px-6 py-4">
                <div>
                  <Label htmlFor="meeting-save-recording" className="text-sm font-medium text-quiet-text-primary">Save meeting recording</Label>
                  <p className="mt-0.5 text-[12.5px] leading-5 text-quiet-text-tertiary">
                    Keep video and audio for private playback in Helpin.
                  </p>
                </div>
                <Switch id="meeting-save-recording" checked={recordAudio} onCheckedChange={setRecordAudioOverride} />
              </div>
            </div>
          </div>
          <DialogFooter className="flex-row items-center justify-end gap-4 border-t border-quiet-divider-strong px-6 py-4">
            <QuietTextAction onClick={() => handleOpenChange(false)}>
              Close without adding
            </QuietTextAction>
            <QuietPrimaryAction onClick={submit} disabled={createMeeting.isPending || !title.trim() || !meetingUrl.trim() || (startNow && !settingsEnabled)}>
              {createMeeting.isPending ? 'Creating…' : startNow ? 'Create & join' : 'Schedule meeting'}
            </QuietPrimaryAction>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <UpgradeRequiredDialog
        open={upgradeReason !== null}
        onOpenChange={(dialogOpen) => {
          if (!dialogOpen) setUpgradeReason(null);
        }}
        reason={upgradeReason}
      />
    </>
  );
}
