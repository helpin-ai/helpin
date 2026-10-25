import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { QuietIconAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Cancel01Icon, InformationCircleIcon, PlusSignIcon } from '@/lib/icons';
import { useUpdateChatSettings } from '@/hooks/queries/useSupport';
import type { SupportInstallationResponse, WidgetOriginSettings as OriginSettings } from '@/lib/pmTypes';
import { WidgetSigningSecret } from './WidgetSigningSecret';
import { INVALID_ORIGIN, normalizeWidgetOrigin } from './widgetOrigins';

interface OriginRow {
  id: number;
  value: string;
}

let nextOriginRowId = 0;

function toRows(values: string[]): OriginRow[] {
  return (values.length ? values : ['']).map(value => ({ id: nextOriginRowId++, value }));
}

/** Saved origins to edit, or the website suggestion while the widget is blocked. */
function initialOrigins(allowed: string[] | undefined, suggested: string[] | undefined): string[] {
  const saved = (allowed ?? []).filter(origin => origin !== '*');
  if (saved.length || allowed?.includes('*')) return saved;
  return suggested ?? [];
}

function FieldHelp({ label, children }: { label: string; children: string }) {
  return (
    <QuickTooltip label={children} contentClassName="max-w-72">
      <button type="button" aria-label={label} className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-quiet-text-tertiary hover:text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
        <InformationCircleIcon className="size-3.5" />
      </button>
    </QuickTooltip>
  );
}

export function WidgetOriginSettings({ workspaceId, installation, canManageSigningSecret = false }: {
  workspaceId: string;
  installation: SupportInstallationResponse;
  /** Requires the support.admin permission, matching the reveal/rotate endpoints. */
  canManageSigningSecret?: boolean;
}) {
  const [rows, setRows] = useState(() => toRows(initialOrigins(installation.allowed_origins, installation.suggested_allowed_origins)));
  const [allowAll, setAllowAll] = useState(installation.allowed_origins?.includes('*') ?? false);
  const [mode, setMode] = useState<OriginSettings['identity_verification_mode']>(installation.identity_verification_mode);
  const [dirty, setDirty] = useState(false);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});
  const [error, setError] = useState('');
  const mutation = useUpdateChatSettings(workspaceId);
  const blocked = !installation.allowed_origins?.length;
  const suggested = blocked && (installation.suggested_allowed_origins?.length ?? 0) > 0;
  useEffect(() => {
    if (!dirty) {
      setRows(toRows(initialOrigins(installation.allowed_origins, installation.suggested_allowed_origins)));
      setAllowAll(installation.allowed_origins?.includes('*') ?? false);
      setMode(installation.identity_verification_mode);
    }
  }, [installation.allowed_origins, installation.suggested_allowed_origins, installation.identity_verification_mode, dirty]);

  function edit(next: (current: OriginRow[]) => OriginRow[]) {
    setRows(current => {
      const updated = next(current);
      return updated.length ? updated : toRows([]);
    });
    setDirty(true);
  }

  async function save() {
    setError('');
    const errors: Record<number, string> = {};
    const origins: string[] = [];
    for (const row of rows) {
      if (!row.value.trim()) continue;
      try {
        origins.push(normalizeWidgetOrigin(row.value));
      } catch (cause) {
        errors[row.id] = cause instanceof Error ? cause.message : INVALID_ORIGIN;
      }
    }
    setRowErrors(errors);
    if (Object.keys(errors).length) return;
    const unique = [...new Set(origins)].sort();
    const allowedOrigins = allowAll ? ['*', ...unique] : unique;
    if (mode === 'enforced' && !allowedOrigins.length) {
      setError('Add your website before requiring signed identities.');
      return;
    }
    try {
      await mutation.mutateAsync({ allowed_origins: allowedOrigins, identity_verification_mode: mode });
      setDirty(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Website settings could not be saved.');
    }
  }

  const rowsDisabled = allowAll || mutation.isPending;
  return (
    <div className="space-y-5">
      {blocked && !allowAll && (
        <p role="status" className="text-sm text-quiet-accent">
          {suggested ? 'Filled in from your workspace website. Save to turn on the widget.' : 'Add at least one website to turn on the widget.'}
        </p>
      )}
      <div className="space-y-2">
        <div className="flex items-center gap-1">
          <span id="widget-origins-label" className="text-sm font-medium">Websites</span>
          <FieldHelp label="About websites">
            One website per row, like example.com. *.example.com adds every subdomain. Add test sites such as localhost:3000 on their own row. Tauri desktop apps use tauri://localhost.
          </FieldHelp>
        </div>
        <div role="group" aria-labelledby="widget-origins-label" className="space-y-1">
          {rows.map((row, index) => (
            <div key={row.id}>
              <div className="flex items-center gap-1">
                <QuietUnderlineInput
                  value={row.value}
                  placeholder={index === 0 ? 'example.com' : '*.example.com'}
                  aria-label={`Website ${index + 1}`}
                  aria-invalid={rowErrors[row.id] ? true : undefined}
                  aria-describedby={rowErrors[row.id] ? `widget-origin-error-${row.id}` : undefined}
                  disabled={rowsDisabled}
                  autoComplete="off"
                  spellCheck={false}
                  className="min-w-0 flex-1"
                  onChange={event => {
                    const value = event.target.value;
                    edit(current => current.map(candidate => candidate.id === row.id ? { ...candidate, value } : candidate));
                    setRowErrors(current => {
                      if (!(row.id in current)) return current;
                      const next = { ...current };
                      delete next[row.id];
                      return next;
                    });
                  }}
                />
                {(rows.length > 1 || row.value) && (
                  <QuickTooltip label="Remove">
                    <QuietIconAction type="button" aria-label={`Remove website ${index + 1}`} disabled={rowsDisabled}
                      onClick={() => edit(current => current.filter(candidate => candidate.id !== row.id))}>
                      <Cancel01Icon className="h-4 w-4" />
                    </QuietIconAction>
                  </QuickTooltip>
                )}
              </div>
              {rowErrors[row.id] && <p id={`widget-origin-error-${row.id}`} role="alert" className="pt-1 text-xs text-destructive">{rowErrors[row.id]}</p>}
            </div>
          ))}
        </div>
        <QuietTextAction type="button" disabled={rowsDisabled} onClick={() => edit(current => [...current, ...toRows([''])])}>
          <PlusSignIcon className="h-3.5 w-3.5" />
          Add website
        </QuietTextAction>
      </div>
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <Checkbox id="widget-allow-all-origins" checked={allowAll} disabled={mutation.isPending} onCheckedChange={value => { setAllowAll(value === true); setDirty(true); }} />
          <Label htmlFor="widget-allow-all-origins">Allow any website or app</Label>
        </div>
        {allowAll && <p className="text-xs text-quiet-text-secondary">Any site can load your widget, including apps that send no origin.</p>}
      </div>
      <div className="space-y-2">
        <div className="flex items-center gap-1">
          <Label htmlFor="widget-identity-mode">Visitor identity</Label>
          <FieldHelp label="About visitor identity">
            Signed identities need your server to sign each logged-in visitor with the signing secret below. Visitors without a valid signature can still chat anonymously.
          </FieldHelp>
        </div>
        <Select value={mode} disabled={mutation.isPending} onValueChange={value => { setMode(value as OriginSettings['identity_verification_mode']); setDirty(true); }}>
          <SelectTrigger id="widget-identity-mode" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="report_only">Accept unverified identities</SelectItem>
            <SelectItem value="enforced">Require server-signed identities</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <Button type="button" size="sm" disabled={!(dirty || suggested) || mutation.isPending} onClick={() => void save()}>{mutation.isPending ? 'Saving…' : 'Save'}</Button>
      <WidgetSigningSecret
        workspaceId={workspaceId}
        configured={installation.signing_secret_configured ?? true}
        canManage={canManageSigningSecret}
        enforced={installation.identity_verification_mode === 'enforced'}
      />
    </div>
  );
}
