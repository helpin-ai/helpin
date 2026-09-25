import { useEffect, useState } from 'react';
import {
  useChatSettings,
  useUpdateChatSettings,
} from '@/hooks/queries/useSupport';
import { useSettingsAutosave } from '@/hooks/useSettingsAutosave';
import { Card, CardContent } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { SettingsSaveBar } from './SettingsSaveBar';
import { SettingsSaveStatus } from './SettingsSaveStatus';
import { SettingsAutosaveGuard } from './SettingsAutosaveGuard';
import { TranslationLanguagePicker } from './TranslationLanguagePicker';
import { TRANSLATION_LANGUAGES } from './translationLanguages';

export function SupportTranslationSettings(props: {
  workspaceId: string;
  editable: boolean;
}) {
  return <TranslationSettingsEditor key={props.workspaceId} {...props} />;
}

function TranslationSettingsEditor({
  workspaceId,
  editable,
}: {
  workspaceId: string;
  editable: boolean;
}) {
  const query = useChatSettings(workspaceId);
  const update = useUpdateChatSettings(workspaceId);
  const [hydrated, setHydrated] = useState(false);
  const [draft, setDraft] = useState({
    translation_enabled: true,
    default_agent_language: 'en',
  });
  useEffect(() => {
    if (!query.data || hydrated) return;
    const settings = query.data.settings;
    setDraft({
      translation_enabled: settings.translation_enabled ?? true,
      default_agent_language: settings.default_agent_language || 'en',
    });
    setHydrated(true);
  }, [query.data, hydrated]);
  const autosave = useSettingsAutosave({
    scopeKey: workspaceId,
    enabled: hydrated && editable,
    value: draft,
    savedValue: hydrated ? draft : null,
    save: (value) => update.mutateAsync(value),
  });
  if (!hydrated) {
    if (query.isError)
      return (
        <div role="alert" className="space-y-3 text-sm">
          <p>Could not load translation settings.</p>
          <Button variant="outline" onClick={() => void query.refetch()}>
            Retry
          </Button>
        </div>
      );
    return <Skeleton className="h-64" />;
  }
  const disabled = !editable;
  return (
    <div className="space-y-4">
      {editable ? (
        <SettingsSaveBar>
          <SettingsAutosaveGuard
            isDirty={autosave.isDirty}
            error={autosave.error}
            onRetry={autosave.retry}
          />
          <span className="text-xs text-muted-foreground">
            Changes save automatically
          </span>
          <SettingsSaveStatus
            status={autosave.status}
            error={autosave.error}
            onRetry={autosave.retry}
          />
        </SettingsSaveBar>
      ) : (
        <p className="text-sm text-muted-foreground">
          A workspace support administrator can change these settings.
        </p>
      )}
      <Card>
        <CardContent className="space-y-5">
          <TranslationLanguagePicker label="Reading language" value={draft.default_agent_language} languages={TRANSLATION_LANGUAGES} disabled={disabled} onChange={value=>setDraft(d=>({...d,default_agent_language:value}))}/>
          <div className="flex items-center justify-between gap-4 border-t border-border/50 pt-5">
            <Label htmlFor="translation-enabled" title="Enable Live Translate for new conversations. Existing conversations keep their setting.">Live Translate on by default</Label>
            <Switch id="translation-enabled" checked={draft.translation_enabled} disabled={disabled} onCheckedChange={value=>setDraft(d=>({...d,translation_enabled:value}))}/>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
