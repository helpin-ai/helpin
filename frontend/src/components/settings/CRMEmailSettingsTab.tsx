import { SettingsSaveBar } from './SettingsSaveBar';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { QuietPrimaryAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { CRMEmailSettingsSection } from '@/components/crm/CRMEmailSettingsSection';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useEmailSyncSettings, useUpdateEmailSyncSettings } from '@/hooks/queries/useCRM';
import { Cancel01Icon, InformationCircleIcon, FloppyDiskIcon, PlusSignIcon, RotateLeft01Icon, Tick01Icon } from '@/lib/icons';
import { unwrap } from '@/lib/queryUtils';
import { crmEmailSyncSettingsService } from '@/lib/services/crmService';
import type {
  CRMEmailSyncSettings,
  CRMFilterMode,
  CRMInternalExclusion,
  CRMRecordCreationMode,
} from '@/lib/crmTypes';
import { useAuthStore } from '@/stores/authStore';

function RemovableTag({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <Badge variant="secondary" className="group/tag gap-1 pr-1 font-mono text-[11px] font-normal">
      {label}
      <button
        type="button"
        aria-label={`Remove ${label}`}
        onClick={onRemove}
        className="ml-0.5 rounded-full p-0.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </Badge>
  );
}

function SettingRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_280px] sm:items-center">
      <div>
        <div className="flex items-center gap-1">
          <Label className="text-sm font-medium">{label}</Label>
          {description && <QuickTooltip label={description}><button type="button" aria-label={`About ${label.toLowerCase()}`} className="inline-flex size-5 items-center justify-center rounded-sm text-quiet-text-secondary focus-visible:outline-2 focus-visible:outline-ring"><InformationCircleIcon className="size-3.5" /></button></QuickTooltip>}
        </div>
      </div>
      <div className="sm:justify-self-end">{children}</div>
    </div>
  );
}

export function CRMEmailSettingsTab({ workspaceId }: { workspaceId: string }) {
  const { data: settings, isLoading, isError } = useEmailSyncSettings(workspaceId);

  if (isLoading && !isError) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-56 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  return (
    <CRMEmailSettingsContent
      workspaceId={workspaceId}
      settings={settings}
      isError={isError}
    />
  );
}

function CRMEmailSettingsContent({
  workspaceId,
  settings,
  isError,
}: {
  workspaceId: string;
  settings?: CRMEmailSyncSettings;
  isError: boolean;
}) {
  const user = useAuthStore((state) => state.user);
  const updateSettings = useUpdateEmailSyncSettings(workspaceId);
  const [saved, setSaved] = useState(false);
  const [defaultsLoaded, setDefaultsLoaded] = useState(Boolean(settings));

  const [historicalSyncDays, setHistoricalSyncDays] = useState(settings?.historical_sync_days ?? 90);
  const [filterMode, setFilterMode] = useState<CRMFilterMode>(settings?.filter_mode ?? 'blocklist');
  const [filterPatterns, setFilterPatterns] = useState<string[]>(settings?.filter_patterns ?? []);
  const [internalExclusion, setInternalExclusion] = useState<CRMInternalExclusion>(settings?.internal_exclusion ?? 'none');
  const [includePrivateMeetings, setIncludePrivateMeetings] = useState(settings?.include_private_meetings ?? false);
  const [includeSoloMeetings, setIncludeSoloMeetings] = useState(settings?.include_solo_meetings ?? false);
  const [recordCreationMode, setRecordCreationMode] = useState<CRMRecordCreationMode>(settings?.record_creation_mode ?? 'selective');
  const [blockedRecordPrefixes, setBlockedRecordPrefixes] = useState<string[]>(settings?.blocked_record_prefixes ?? []);
  const [newPattern, setNewPattern] = useState('');
  const [newPrefix, setNewPrefix] = useState('');

  useEffect(() => {
    if (!isError || defaultsLoaded) return;
    crmEmailSyncSettingsService.getDefaultPrefixes()
      .then((response) => {
        if (response.data) {
          setBlockedRecordPrefixes(response.data);
          setDefaultsLoaded(true);
        }
      })
      .catch(() => {});
  }, [defaultsLoaded, isError]);

  const isDirty = useMemo(() => {
    if (!settings) return isError && defaultsLoaded;
    return (
      historicalSyncDays !== settings.historical_sync_days
      || filterMode !== settings.filter_mode
      || JSON.stringify(filterPatterns) !== JSON.stringify(settings.filter_patterns ?? [])
      || internalExclusion !== settings.internal_exclusion
      || includePrivateMeetings !== settings.include_private_meetings
      || includeSoloMeetings !== settings.include_solo_meetings
      || recordCreationMode !== settings.record_creation_mode
      || JSON.stringify(blockedRecordPrefixes) !== JSON.stringify(settings.blocked_record_prefixes ?? [])
    );
  }, [
    blockedRecordPrefixes,
    defaultsLoaded,
    filterMode,
    filterPatterns,
    historicalSyncDays,
    includePrivateMeetings,
    includeSoloMeetings,
    internalExclusion,
    isError,
    recordCreationMode,
    settings,
  ]);

  const handleSave = async () => {
    try {
      await updateSettings.mutateAsync({
        historical_sync_days: historicalSyncDays,
        filter_mode: filterMode,
        filter_patterns: filterPatterns,
        internal_exclusion: internalExclusion,
        include_private_meetings: includePrivateMeetings,
        include_solo_meetings: includeSoloMeetings,
        record_creation_mode: recordCreationMode,
        blocked_record_prefixes: blockedRecordPrefixes,
      });
      setSaved(true);
      toast.success('Email preferences saved');
      window.setTimeout(() => setSaved(false), 2000);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to save settings');
    }
  };

  const addPattern = () => {
    const pattern = newPattern.trim().toLowerCase();
    if (!pattern || filterPatterns.includes(pattern)) return;
    setFilterPatterns([...filterPatterns, pattern]);
    setNewPattern('');
  };

  const addPrefix = () => {
    const prefix = newPrefix.trim().toLowerCase();
    if (!prefix || blockedRecordPrefixes.includes(prefix)) return;
    setBlockedRecordPrefixes([...blockedRecordPrefixes, prefix]);
    setNewPrefix('');
  };

  const resetPrefixesToDefault = async () => {
    try {
      setBlockedRecordPrefixes(unwrap(await crmEmailSyncSettingsService.getDefaultPrefixes()));
      toast.success('Default prefixes restored');
    } catch {
      toast.error('Failed to load default prefixes');
    }
  };

  return (
    <div className="space-y-4 pb-6">
      <SettingsSaveBar visible={isDirty || saved || updateSettings.isPending}>
        {isDirty && <span className="text-xs text-muted-foreground">Unsaved changes</span>}
        <QuietPrimaryAction onClick={() => void handleSave()} disabled={updateSettings.isPending || saved || !isDirty}>
          {saved ? (
            <><Tick01Icon className="mr-1.5 h-4 w-4" />Saved</>
          ) : updateSettings.isPending ? (
            'Saving…'
          ) : (
            <><FloppyDiskIcon className="mr-1.5 h-4 w-4" />Save changes</>
          )}
        </QuietPrimaryAction>
      </SettingsSaveBar>
      {isError && (
        <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2 text-sm text-muted-foreground">
          Saved sync preferences could not be loaded. Review the defaults below and save to initialize them.
        </div>
      )}

      <EmailAccountConnect workspaceId={workspaceId} memberId={user?.id ?? ''} showAll />

      <CRMEmailSettingsSection title="Sync preferences" optionId="email-sync">
        <div className="space-y-5">
          <SettingRow label="Email history" description="Used for the first import and recovery syncs.">
            <Select value={String(historicalSyncDays)} onValueChange={(value) => setHistoricalSyncDays(Number(value))}>
              <SelectTrigger variant="underline" className="w-full sm:w-40"><SelectValue /></SelectTrigger>
              <SelectContent>
                {[30, 60, 90, 180, 365].map((days) => <SelectItem key={days} value={String(days)}>{days} days</SelectItem>)}
              </SelectContent>
            </Select>
          </SettingRow>

          <Separator />

          <SettingRow
            label="Email filter"
            description={filterMode === 'blocklist' ? 'Exclude addresses that match your patterns.' : 'Only sync addresses that match your patterns.'}
          >
            <Select value={filterMode} onValueChange={(value) => setFilterMode(value as CRMFilterMode)}>
              <SelectTrigger variant="underline" className="w-full sm:w-40"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="blocklist">Blocklist</SelectItem>
                <SelectItem value="allowlist">Allowlist</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <div className="space-y-2">
            <Label htmlFor="email-filter-pattern">Address or domain patterns</Label>
            <div className="flex gap-2">
              <QuietUnderlineInput
                id="email-filter-pattern"
                value={newPattern}
                onChange={(event) => setNewPattern(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    addPattern();
                  }
                }}
                placeholder="support@example.com or *@example.org"
                className="font-mono text-sm placeholder:font-sans"
              />
              <Button type="button" variant="outline" size="icon" aria-label="Add email pattern" onClick={addPattern} disabled={!newPattern.trim()}>
                <PlusSignIcon className="h-4 w-4" />
              </Button>
            </div>
            {filterPatterns.length ? (
              <div className="flex flex-wrap gap-1.5 py-2">
                {filterPatterns.map((pattern) => (
                  <RemovableTag key={pattern} label={pattern} onRemove={() => setFilterPatterns(filterPatterns.filter((item) => item !== pattern))} />
                ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">No patterns added.</p>
            )}
          </div>
        </div>
      </CRMEmailSettingsSection>

      <CRMEmailSettingsSection title="Calendar events" optionId="email-calendar">
        <div className="space-y-5">
          <SettingRow label="Internal activity" description="Optionally ignore email and meetings where everyone uses your company domain.">
            <Select value={internalExclusion} onValueChange={(value) => setInternalExclusion(value as CRMInternalExclusion)}>
              <SelectTrigger variant="underline" className="w-full sm:w-64"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="none">Include internal activity</SelectItem>
                <SelectItem value="exclude">Exclude internal activity</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <Separator />

          <SettingRow label="Private meetings" description="Include events marked private in calendar sync.">
            <Switch id="include-private-meetings" checked={includePrivateMeetings} onCheckedChange={setIncludePrivateMeetings} />
          </SettingRow>

          <Separator />

          <SettingRow label="Solo meetings" description="Include events without any other participants.">
            <Switch id="include-solo-meetings" checked={includeSoloMeetings} onCheckedChange={setIncludeSoloMeetings} />
          </SettingRow>
        </div>
      </CRMEmailSettingsSection>

      <CRMEmailSettingsSection title="Contact creation" optionId="email-contacts">
        <div className="space-y-5">
          <SettingRow
            label="Create contacts"
            description={
              recordCreationMode === 'disabled'
                ? 'Never create contacts automatically.'
                : recordCreationMode === 'selective'
                  ? 'Create contacts for outbound email and meeting participants.'
                  : 'Create contacts for every email and meeting participant.'
            }
          >
            <Select value={recordCreationMode} onValueChange={(value) => setRecordCreationMode(value as CRMRecordCreationMode)}>
              <SelectTrigger variant="underline" className="w-full sm:w-44"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="disabled">Never</SelectItem>
                <SelectItem value="selective">Selectively</SelectItem>
                <SelectItem value="always">Always</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <Separator />

          <div className="space-y-2">
            <div className="flex flex-wrap items-end justify-between gap-2">
              <div>
                <div className="flex items-center gap-1"><Label htmlFor="blocked-email-prefix">Blocked email prefixes</Label><QuickTooltip label="Prevent automated addresses such as noreply@ from creating contacts."><button type="button" aria-label="About blocked email prefixes" className="inline-flex size-5 items-center justify-center rounded-sm text-quiet-text-secondary focus-visible:outline-2 focus-visible:outline-ring"><InformationCircleIcon className="size-3.5" /></button></QuickTooltip></div>
              </div>
              <Button type="button" variant="ghost" size="sm" onClick={() => void resetPrefixesToDefault()} className="h-7 text-xs text-muted-foreground">
                <RotateLeft01Icon className="mr-1.5 h-3.5 w-3.5" />Reset defaults
              </Button>
            </div>
            <div className="flex gap-2">
              <QuietUnderlineInput
                id="blocked-email-prefix"
                value={newPrefix}
                onChange={(event) => setNewPrefix(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    addPrefix();
                  }
                }}
                placeholder="noreply, support, billing"
                className="font-mono text-sm placeholder:font-sans"
              />
              <Button type="button" variant="outline" size="icon" aria-label="Add blocked prefix" onClick={addPrefix} disabled={!newPrefix.trim()}>
                <PlusSignIcon className="h-4 w-4" />
              </Button>
            </div>
            {blockedRecordPrefixes.length ? (
              <div className="max-h-44 overflow-y-auto py-2">
                <div className="flex flex-wrap gap-1.5">
                  {blockedRecordPrefixes.map((prefix) => (
                    <RemovableTag key={prefix} label={prefix} onRemove={() => setBlockedRecordPrefixes(blockedRecordPrefixes.filter((item) => item !== prefix))} />
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">No prefixes blocked.</p>
            )}
          </div>
        </div>
      </CRMEmailSettingsSection>


    </div>
  );
}
