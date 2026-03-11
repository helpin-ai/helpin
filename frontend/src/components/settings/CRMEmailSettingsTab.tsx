import { useEffect, useMemo, useState } from 'react';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { useAuthStore } from '@/stores/authStore';
import { useEmailSyncSettings, useUpdateEmailSyncSettings } from '@/hooks/queries/useCRM';
import { crmEmailSyncSettingsService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { X, Plus, RotateCcw, Mail, Clock, Filter, Building2, Users, ShieldCheck, Save, Check } from 'lucide-react';
import { toast } from 'sonner';
import type { CRMFilterMode, CRMRecordCreationMode, CRMInternalExclusion } from '@/lib/crmTypes';

/* ────────────────────────────────────────────────────────
 * Section header with number + title + description
 * ──────────────────────────────────────────────────────── */
function SectionHeader({
  number,
  icon: Icon,
  title,
  description,
}: {
  number: string;
  icon: React.ElementType;
  title: string;
  description: string;
}) {
  return (
    <div className="flex items-start gap-4 pb-4">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/[0.06] text-primary">
        <Icon className="h-[18px] w-[18px]" strokeWidth={1.8} />
      </div>
      <div className="min-w-0">
        <div className="flex items-center gap-2.5">
          <span className="font-mono text-[11px] font-medium tracking-widest text-muted-foreground/60 uppercase">
            {number}
          </span>
          <div className="h-px w-4 bg-border" />
          <h3 className="text-[15px] font-semibold tracking-tight">{title}</h3>
        </div>
        <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────
 * Removable tag / chip for patterns and prefixes
 * ──────────────────────────────────────────────────────── */
function RemovableTag({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <Badge
      variant="secondary"
      className="group/tag gap-1 pr-1 font-mono text-[11px] font-normal tracking-wide transition-all duration-150 hover:bg-destructive/10 hover:text-destructive"
    >
      {label}
      <button
        type="button"
        onClick={onRemove}
        className="ml-0.5 rounded-full p-0.5 opacity-40 transition-opacity group-hover/tag:opacity-100"
      >
        <X className="h-3 w-3" />
      </button>
    </Badge>
  );
}

/* ────────────────────────────────────────────────────────
 * Main component
 * ──────────────────────────────────────────────────────── */
export function CRMEmailSettingsTab({ workspaceId }: { workspaceId: string }) {
  const user = useAuthStore((s) => s.user);
  const { data: settings, isLoading, isError } = useEmailSyncSettings(workspaceId);
  const updateSettings = useUpdateEmailSyncSettings(workspaceId);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [defaultsLoaded, setDefaultsLoaded] = useState(false);

  const [historicalSyncDays, setHistoricalSyncDays] = useState(90);
  const [filterMode, setFilterMode] = useState<CRMFilterMode>('blocklist');
  const [filterPatterns, setFilterPatterns] = useState<string[]>([]);
  const [internalExclusion, setInternalExclusion] = useState<CRMInternalExclusion>('none');
  const [includePrivateMeetings, setIncludePrivateMeetings] = useState(false);
  const [includeSoloMeetings, setIncludeSoloMeetings] = useState(false);
  const [recordCreationMode, setRecordCreationMode] = useState<CRMRecordCreationMode>('selective');
  const [blockedRecordPrefixes, setBlockedRecordPrefixes] = useState<string[]>([]);

  const [newPattern, setNewPattern] = useState('');
  const [newPrefix, setNewPrefix] = useState('');

  useEffect(() => {
    if (settings) {
      setHistoricalSyncDays(settings.historical_sync_days);
      setFilterMode(settings.filter_mode);
      setFilterPatterns(settings.filter_patterns ?? []);
      setInternalExclusion(settings.internal_exclusion);
      setIncludePrivateMeetings(settings.include_private_meetings);
      setIncludeSoloMeetings(settings.include_solo_meetings);
      setRecordCreationMode(settings.record_creation_mode);
      setBlockedRecordPrefixes(settings.blocked_record_prefixes ?? []);
      setDefaultsLoaded(true);
    }
  }, [settings]);

  /* If the settings query fails (e.g. table not migrated yet), prefill with defaults */
  useEffect(() => {
    if (isError && !defaultsLoaded) {
      crmEmailSyncSettingsService.getDefaultPrefixes().then((res) => {
        if (res.data) {
          setBlockedRecordPrefixes(res.data);
          setDefaultsLoaded(true);
        }
      }).catch(() => {});
    }
  }, [isError, defaultsLoaded]);

  /* Dirty detection */
  const isDirty = useMemo(() => {
    if (!settings) return false;
    return (
      historicalSyncDays !== settings.historical_sync_days ||
      filterMode !== settings.filter_mode ||
      JSON.stringify(filterPatterns) !== JSON.stringify(settings.filter_patterns ?? []) ||
      internalExclusion !== settings.internal_exclusion ||
      includePrivateMeetings !== settings.include_private_meetings ||
      includeSoloMeetings !== settings.include_solo_meetings ||
      recordCreationMode !== settings.record_creation_mode ||
      JSON.stringify(blockedRecordPrefixes) !== JSON.stringify(settings.blocked_record_prefixes ?? [])
    );
  }, [
    settings,
    historicalSyncDays,
    filterMode,
    filterPatterns,
    internalExclusion,
    includePrivateMeetings,
    includeSoloMeetings,
    recordCreationMode,
    blockedRecordPrefixes,
  ]);

  const handleSave = async () => {
    setSaving(true);
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
      toast.success('Email sync settings saved');
      setTimeout(() => setSaved(false), 2000);
    } catch {
      toast.error('Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const addPattern = () => {
    const trimmed = newPattern.trim().toLowerCase();
    if (trimmed && !filterPatterns.includes(trimmed)) {
      setFilterPatterns([...filterPatterns, trimmed]);
      setNewPattern('');
    }
  };

  const removePattern = (pattern: string) => {
    setFilterPatterns(filterPatterns.filter((p) => p !== pattern));
  };

  const addPrefix = () => {
    const trimmed = newPrefix.trim().toLowerCase();
    if (trimmed && !blockedRecordPrefixes.includes(trimmed)) {
      setBlockedRecordPrefixes([...blockedRecordPrefixes, trimmed]);
      setNewPrefix('');
    }
  };

  const removePrefix = (prefix: string) => {
    setBlockedRecordPrefixes(blockedRecordPrefixes.filter((p) => p !== prefix));
  };

  const resetPrefixesToDefault = async () => {
    try {
      const defaults = unwrap(await crmEmailSyncSettingsService.getDefaultPrefixes());
      setBlockedRecordPrefixes(defaults);
      toast.success('Reset to default prefixes');
    } catch {
      toast.error('Failed to load default prefixes');
    }
  };

  if (isLoading && !isError) {
    return (
      <div className="space-y-4">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-32 w-full rounded-xl" />
        ))}
      </div>
    );
  }

  return (
    <div className="relative space-y-5 pb-20">
      {/* ── 01 Connected Accounts ────────────────────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="01"
            icon={Mail}
            title="Connected Accounts"
            description="Connect email accounts to sync conversations and detect buyer signals."
          />
          <div className="ml-[52px]">
            <EmailAccountConnect workspaceId={workspaceId} memberId={user?.id ?? ''} showAll />
          </div>
        </CardContent>
      </Card>

      {/* ── 02 Historical Sync ───────────────────────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="02"
            icon={Clock}
            title="Historical Sync Period"
            description="How many days of email history to sync when a new account is connected."
          />
          <div className="ml-[52px] flex items-center gap-4">
            <Select
              value={String(historicalSyncDays)}
              onValueChange={(v) => setHistoricalSyncDays(Number(v))}
            >
              <SelectTrigger className="w-36 font-mono text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[30, 60, 90, 180, 365].map((d) => (
                  <SelectItem key={d} value={String(d)} className="font-mono text-sm">
                    {d} days
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-xs italic text-muted-foreground/70">
              Changes only affect newly connected accounts
            </span>
          </div>
        </CardContent>
      </Card>

      {/* ── 03 Email & Meeting Filtering ─────────────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="03"
            icon={Filter}
            title="Email & Meeting Filtering"
            description="Control which emails are synced using pattern-based filtering."
          />
          <div className="ml-[52px] space-y-5">
            <div className="space-y-2">
              <Label className="text-[13px] font-medium">Filter Mode</Label>
              <div className="flex items-center gap-3">
                <Select value={filterMode} onValueChange={(v) => setFilterMode(v as CRMFilterMode)}>
                  <SelectTrigger className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="blocklist">Blocklist</SelectItem>
                    <SelectItem value="allowlist">Allowlist</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {filterMode === 'blocklist'
                    ? 'Matching patterns will be excluded from sync.'
                    : 'Only matching patterns will be synced.'}
                </p>
              </div>
            </div>

            <div className="space-y-2.5">
              <Label className="text-[13px] font-medium">Patterns</Label>
              <div className="flex gap-2">
                <Input
                  placeholder="e.g. support@example.com, *@example.org"
                  value={newPattern}
                  onChange={(e) => setNewPattern(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      addPattern();
                    }
                  }}
                  className="flex-1 font-mono text-sm placeholder:font-sans"
                />
                <Button variant="outline" size="icon" onClick={addPattern} disabled={!newPattern.trim()}>
                  <Plus className="h-4 w-4" />
                </Button>
              </div>
              {filterPatterns.length > 0 && (
                <div className="flex flex-wrap gap-1.5 rounded-lg border border-dashed border-border/80 bg-muted/30 p-3">
                  {filterPatterns.map((pattern) => (
                    <RemovableTag key={pattern} label={pattern} onRemove={() => removePattern(pattern)} />
                  ))}
                </div>
              )}
              {filterPatterns.length === 0 && (
                <p className="py-2 text-center text-xs italic text-muted-foreground/50">
                  No patterns configured — all emails will be synced
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── 04 Internal Communication & Meetings ─────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="04"
            icon={Building2}
            title="Internal Communication & Meetings"
            description="Configure how internal emails and meetings are handled."
          />
          <div className="ml-[52px] space-y-5">
            <div className="space-y-2">
              <Label className="text-[13px] font-medium">Internal Exclusion</Label>
              <Select
                value={internalExclusion}
                onValueChange={(v) => setInternalExclusion(v as CRMInternalExclusion)}
              >
                <SelectTrigger className="w-72">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">None</SelectItem>
                  <SelectItem value="exclude">Exclude internal emails and meetings</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                When enabled, emails where all participants share the same domain as the connected account are excluded.
              </p>
            </div>

            <div className="h-px bg-border/60" />

            <div className="flex items-center justify-between py-0.5">
              <div>
                <Label className="text-[13px] font-medium">Include private meetings</Label>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Enable ingestion of meetings marked as private
                </p>
              </div>
              <Switch checked={includePrivateMeetings} onCheckedChange={setIncludePrivateMeetings} />
            </div>

            <div className="flex items-center justify-between py-0.5">
              <div>
                <Label className="text-[13px] font-medium">Include solo meetings</Label>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Enable ingestion of meetings with no additional participants
                </p>
              </div>
              <Switch checked={includeSoloMeetings} onCheckedChange={setIncludeSoloMeetings} />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── 05 Record Creation ───────────────────────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="05"
            icon={Users}
            title="Record Creation"
            description="Control when new contact records are created from synced emails and calendar meetings."
          />
          <div className="ml-[52px] space-y-4">
            <Select
              value={recordCreationMode}
              onValueChange={(v) => setRecordCreationMode(v as CRMRecordCreationMode)}
            >
              <SelectTrigger className="w-44">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="disabled">Disabled</SelectItem>
                <SelectItem value="selective">Selective</SelectItem>
                <SelectItem value="always">Always</SelectItem>
              </SelectContent>
            </Select>
            <div className="space-y-1 rounded-lg bg-muted/40 px-4 py-3 text-xs text-muted-foreground">
              <div className="flex items-baseline gap-2">
                <span className="inline-block w-16 shrink-0 font-mono font-medium text-foreground/70">
                  disabled
                </span>
                <span>Never auto-create contacts from synced emails</span>
              </div>
              <div className="flex items-baseline gap-2">
                <span className="inline-block w-16 shrink-0 font-mono font-medium text-foreground/70">
                  selective
                </span>
                <span>Create for outbound emails and all meeting participants</span>
              </div>
              <div className="flex items-baseline gap-2">
                <span className="inline-block w-16 shrink-0 font-mono font-medium text-foreground/70">
                  always
                </span>
                <span>Create for any email or meeting participant</span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── 06 Blocked Record Creation Prefixes ──────────── */}
      <Card className="overflow-hidden border-0 shadow-sm ring-1 ring-border/60">
        <CardContent className="p-6">
          <SectionHeader
            number="06"
            icon={ShieldCheck}
            title="Blocked Record Creation Prefixes"
            description="Email prefixes (the part before @) blocked from creating records. Typically automated or system emails."
          />
          <div className="ml-[52px] space-y-3">
            <div className="flex gap-2">
              <Input
                placeholder="e.g. noreply, support, billing"
                value={newPrefix}
                onChange={(e) => setNewPrefix(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    addPrefix();
                  }
                }}
                className="flex-1 font-mono text-sm placeholder:font-sans"
              />
              <Button variant="outline" size="icon" onClick={addPrefix} disabled={!newPrefix.trim()}>
                <Plus className="h-4 w-4" />
              </Button>
            </div>

            {blockedRecordPrefixes.length > 0 && (
              <div className="max-h-52 overflow-y-auto rounded-lg border border-dashed border-border/80 bg-muted/30 p-3">
                <div className="flex flex-wrap gap-1">
                  {blockedRecordPrefixes.map((prefix) => (
                    <RemovableTag key={prefix} label={prefix} onRemove={() => removePrefix(prefix)} />
                  ))}
                </div>
              </div>
            )}

            <div className="flex items-center justify-between pt-1">
              <p className="font-mono text-[11px] tabular-nums text-muted-foreground">
                {blockedRecordPrefixes.length} prefix{blockedRecordPrefixes.length !== 1 ? 'es' : ''} blocked
              </p>
              <Button
                variant="ghost"
                size="sm"
                onClick={resetPrefixesToDefault}
                className="gap-1.5 text-xs text-muted-foreground hover:text-foreground"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Reset to defaults
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Floating Save Bar ────────────────────────────── */}
      <div
        className={`fixed inset-x-0 bottom-0 z-50 flex items-center justify-end border-t bg-background/80 px-6 py-3 backdrop-blur-md transition-all duration-300 ${
          isDirty ? 'translate-y-0 opacity-100' : 'pointer-events-none translate-y-full opacity-0'
        }`}
      >
        <div className="flex items-center gap-3">
          <span className="text-xs text-muted-foreground">You have unsaved changes</span>
          <Button onClick={handleSave} disabled={saving || saved} className="min-w-[120px] gap-2">
            {saved ? (
              <>
                <Check className="h-4 w-4" />
                Saved
              </>
            ) : saving ? (
              'Saving...'
            ) : (
              <>
                <Save className="h-4 w-4" />
                Save Settings
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}
