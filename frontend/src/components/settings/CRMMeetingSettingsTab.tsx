import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useCRMMeetingSettings, useUpdateCRMMeetingSettings } from '@/hooks/queries/useCRMMeetings';
import type { CRMMeetingSettingsResponse } from '@/lib/crmMeetingTypes';

export function CRMMeetingSettingsTab({ workspaceId, canManage }: { workspaceId: string; canManage: boolean }) {
  const { data, error, isFetching, isLoading, refetch } = useCRMMeetingSettings(workspaceId);

  if (isLoading) return <Skeleton className="h-80 w-full" />;
  if (error || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Unable to load meeting settings</CardTitle>
          <CardDescription>The server could not return meeting settings. Try again now that the API is available.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" onClick={() => void refetch()} disabled={isFetching}>
            {isFetching ? 'Retrying…' : 'Retry'}
          </Button>
        </CardContent>
      </Card>
    );
  }

  const formKey = `${data.settings.updated_at ?? 'defaults'}:${data.settings.enabled}`;
  return <CRMMeetingSettingsForm key={formKey} workspaceId={workspaceId} canManage={canManage} data={data} />;
}

function CRMMeetingSettingsForm({
  workspaceId,
  canManage,
  data,
}: {
  workspaceId: string;
  canManage: boolean;
  data: CRMMeetingSettingsResponse;
}) {
  const updateSettings = useUpdateCRMMeetingSettings(workspaceId);
  const [enabled, setEnabled] = useState(data.settings.enabled);
  const [botName, setBotName] = useState(data.settings.bot_name);
  const [recordAudio, setRecordAudio] = useState(data.settings.record_audio_by_default);

  const save = async () => {
    try {
      await updateSettings.mutateAsync({
        enabled,
        bot_name: botName.trim(),
        record_audio_by_default: recordAudio,
      });
      toast.success('Meeting intelligence settings saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to save meeting settings');
    }
  };

  return (
    <div className="space-y-4">
      {!canManage && (
        <div className="rounded-lg border bg-muted/20 p-3 text-sm text-muted-foreground">
          You can view these settings. A CRM admin is required to change them.
        </div>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Meeting capture</CardTitle>
          <CardDescription>
            Capture meeting transcripts and turn them into summaries, buyer signals, follow-ups, and reviewable project tasks.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between gap-6">
            <div>
              <Label>Enable meeting intelligence</Label>
              <p className="mt-1 text-xs text-muted-foreground">Allows members to invite the Helpin notetaker from the Meetings page.</p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} disabled={!canManage} />
          </div>

          <Separator />

          <div className="space-y-2">
            <Label htmlFor="meeting-bot-name">Notetaker name</Label>
            <Input id="meeting-bot-name" value={botName} onChange={(event) => setBotName(event.target.value)} disabled={!canManage || !enabled} maxLength={100} />
            <p className="text-xs text-muted-foreground">Displayed to participants when the bot joins.</p>
          </div>

          <div className="grid gap-5 md:grid-cols-2">
            <div className="rounded-lg border p-4">
              <Label>Workspace visibility</Label>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">Meeting transcripts and intelligence follow CRM workspace access. Participant/private policies will be enabled only with identity-level enforcement.</p>
            </div>
            <div className="flex items-center justify-between rounded-lg border p-4">
              <div>
                <Label>Record audio by default</Label>
                <p className="mt-1 text-xs text-muted-foreground">Audio is private and can be deleted without deleting the transcript.</p>
              </div>
              <Switch checked={recordAudio} onCheckedChange={setRecordAudio} disabled={!canManage || !enabled} />
            </div>
          </div>
        </CardContent>
      </Card>

      {canManage && (
        <div className="flex justify-end">
          <Button onClick={save} disabled={updateSettings.isPending || !botName.trim()}>
            {updateSettings.isPending ? 'Saving…' : 'Save settings'}
          </Button>
        </div>
      )}
    </div>
  );
}
