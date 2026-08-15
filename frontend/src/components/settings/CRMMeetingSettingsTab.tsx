import { useMemo, useState } from 'react';
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
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function CRMMeetingSettingsTab({ workspaceId, canManage }: { workspaceId: string; canManage: boolean }) {
  const { data, error, isFetching, isLoading, refetch } = useCRMMeetingSettings(workspaceId);

  if (isLoading) return <Skeleton className="h-80 w-full" />;
  if (error || !data) {
    return (
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Unable to load meeting notes settings</CardTitle>
          <CardDescription>Helpin could not load these settings. Check the API connection and try again.</CardDescription>
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
  const isDirty = useMemo(() => (
    enabled !== data.settings.enabled
    || botName.trim() !== data.settings.bot_name
    || recordAudio !== data.settings.record_audio_by_default
  ), [botName, data.settings, enabled, recordAudio]);

  const save = async () => {
    try {
      await updateSettings.mutateAsync({
        enabled,
        bot_name: botName.trim(),
        record_audio_by_default: recordAudio,
      });
      toast.success('Meeting notes settings saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to save meeting notes settings');
    }
  };

  return (
    <div className="space-y-4">
      {!canManage && (
        <div className="rounded-lg border bg-muted/20 p-3 text-sm text-muted-foreground">
          You can view these settings. A CRM admin is required to make changes.
        </div>
      )}

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Meeting notes</CardTitle>
          <CardDescription>Configure how Helpin joins and records meetings.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="flex items-center justify-between gap-6">
            <Label htmlFor="meeting-notes-enabled" className="text-sm font-medium">Enable meeting notes</Label>
            <Switch id="meeting-notes-enabled" checked={enabled} onCheckedChange={setEnabled} disabled={!canManage} />
          </div>

          <Separator />

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_280px] sm:items-center">
            <Label htmlFor="meeting-bot-name" className="text-sm font-medium">Notetaker name</Label>
            <Input id="meeting-bot-name" value={botName} onChange={(event) => setBotName(event.target.value)} disabled={!canManage || !enabled} maxLength={100} placeholder="Helpin Notetaker" />
          </div>

          <Separator />

          <div className="flex items-center justify-between gap-6">
            <Label htmlFor="record-meeting-audio" className="text-sm font-medium">Save audio recordings</Label>
            <Switch id="record-meeting-audio" checked={recordAudio} onCheckedChange={setRecordAudio} disabled={!canManage || !enabled} />
          </div>
        </CardContent>
      </Card>

      {canManage && (
        <div className="flex justify-end">
          <Button onClick={save} disabled={updateSettings.isPending || !botName.trim() || !isDirty}>
            {updateSettings.isPending ? 'Saving…' : isDirty ? 'Save changes' : 'Saved'}
          </Button>
        </div>
      )}
    </div>
  );
}
