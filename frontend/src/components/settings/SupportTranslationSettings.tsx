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
    translation_incoming_enabled: true,
    translation_outgoing_enabled: true,
    default_agent_language: 'en',
    translation_customer_language: '',
  });
  useEffect(() => {
    if (!query.data || hydrated) return;
    const settings = query.data.settings;
    setDraft({
      translation_enabled: settings.translation_enabled ?? true,
      translation_incoming_enabled:
        settings.translation_incoming_enabled ?? true,
      translation_outgoing_enabled:
        settings.translation_outgoing_enabled ?? true,
      default_agent_language: settings.default_agent_language || 'en',
      translation_customer_language:
        settings.translation_customer_language || '',
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
  const disabled = !editable || !draft.translation_enabled;
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
        <CardContent className="space-y-6">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <Label htmlFor="translation-enabled">Auto-translate</Label>
              <p className="text-sm text-muted-foreground">
                Applies to all teammates and support conversations in this
                workspace.
              </p>
              <p className="text-xs text-muted-foreground">
                Requires a configured AI provider. Without one, messages are
                sent and displayed in their original language.
              </p>
            </div>
            <Switch
              id="translation-enabled"
              checked={draft.translation_enabled}
              disabled={!editable}
              onCheckedChange={(value) =>
                setDraft((d) => ({ ...d, translation_enabled: value }))
              }
            />
          </div>
          <div className="space-y-3 border-t border-quiet-divider pt-5">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-1">
                <Label htmlFor="translation-incoming">
                  Translate incoming messages
                </Label>
                <p className="text-sm text-muted-foreground">
                  Show customer messages in the workspace reading language.
                  Teammates can still view the original.
                </p>
              </div>
              <Switch
                id="translation-incoming"
                checked={draft.translation_incoming_enabled}
                disabled={disabled}
                onCheckedChange={(value) =>
                  setDraft((d) => ({
                    ...d,
                    translation_incoming_enabled: value,
                  }))
                }
              />
            </div>
            <TranslationLanguagePicker
              label="Reading language"
              value={draft.default_agent_language}
              languages={TRANSLATION_LANGUAGES}
              disabled={disabled || !draft.translation_incoming_enabled}
              onChange={(value) =>
                setDraft((d) => ({ ...d, default_agent_language: value }))
              }
            />
          </div>
          <div className="space-y-3 border-t border-quiet-divider pt-5">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-1">
                <Label htmlFor="translation-outgoing">Translate replies</Label>
                <p className="text-sm text-muted-foreground">
                  Translate teammate replies when sent. Internal notes stay in
                  their original language.
                </p>
              </div>
              <Switch
                id="translation-outgoing"
                checked={draft.translation_outgoing_enabled}
                disabled={disabled}
                onCheckedChange={(value) =>
                  setDraft((d) => ({
                    ...d,
                    translation_outgoing_enabled: value,
                  }))
                }
              />
            </div>
            <TranslationLanguagePicker
              label="Customer language"
              value={draft.translation_customer_language}
              languages={TRANSLATION_LANGUAGES}
              allowAuto
              disabled={disabled || !draft.translation_outgoing_enabled}
              onChange={(value) =>
                setDraft((d) => ({
                  ...d,
                  translation_customer_language: value,
                }))
              }
            />
            <p className="text-xs text-muted-foreground">
              Detect automatically uses each conversation’s customer language. A
              selected language applies to every conversation.
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
