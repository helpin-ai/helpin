import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useUpdateChatSettings } from '@/hooks/queries/useSupport';
import type { SupportInstallationResponse, WidgetOriginSettings as OriginSettings } from '@/lib/pmTypes';
import { WidgetSigningSecret } from './WidgetSigningSecret';

export function parseWidgetOrigins(value: string): string[] {
  return [...new Set(value.split('\n').map(line => line.trim().replace(/\/$/, '')).filter(Boolean).map(origin => {
    // URL.origin serializes custom schemes as "null". Preserve only the exact
    // supported Tauri origin instead of saving that opaque value.
    if (origin.toLowerCase() === 'tauri://localhost') return 'tauri://localhost';
    const url = new URL(origin);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password ||
      url.pathname !== '/' || url.search || url.hash || /[?#*]/.test(origin) || origin.endsWith('/')) {
      throw new Error('Use an HTTP(S) origin or tauri://localhost, without a path or wildcard.');
    }
    return url.origin;
  }))].sort();
}

export function WidgetOriginSettings({ workspaceId, installation, canManageSigningSecret = false }: {
  workspaceId: string;
  installation: SupportInstallationResponse;
  /** Requires the support.admin permission, matching the reveal/rotate endpoints. */
  canManageSigningSecret?: boolean;
}) {
  const [origins, setOrigins] = useState((installation.allowed_origins ?? []).filter(origin => origin !== '*').join('\n'));
  const [allowAll, setAllowAll] = useState(installation.allowed_origins?.includes('*') ?? false);
  const [mode, setMode] = useState<OriginSettings['identity_verification_mode']>(installation.identity_verification_mode);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState('');
  const mutation = useUpdateChatSettings(workspaceId);
  useEffect(() => {
    if (!dirty) {
      setOrigins((installation.allowed_origins ?? []).filter(origin => origin !== '*').join('\n'));
      setAllowAll(installation.allowed_origins?.includes('*') ?? false);
      setMode(installation.identity_verification_mode);
    }
  }, [installation.allowed_origins, installation.identity_verification_mode, dirty]);
  async function save() {
    setError('');
    try {
      const allowedOrigins = allowAll ? ['*', ...parseWidgetOrigins(origins)] : parseWidgetOrigins(origins);
      if (mode === 'enforced' && !allowedOrigins.length) throw new Error('Add your website or desktop app origin before requiring signed identities.');
      await mutation.mutateAsync({ allowed_origins: allowedOrigins, identity_verification_mode: mode });
      setDirty(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Origin settings could not be saved.');
    }
  }
  return (
    <section className="space-y-4 border-b border-quiet-divider-strong px-4 py-6" aria-labelledby="widget-origin-title">
      <h3 id="widget-origin-title" className="text-sm font-medium">1. Choose where your widget can connect</h3>
      <p className="text-sm text-muted-foreground">
        Restrict access to specific website or desktop app origins, or allow all origins. Include the scheme and any non-default port, one origin per line.
        Add local test and preview origins explicitly. When restricted, an empty list blocks all visitor access.
      </p>
      <p className="text-xs text-muted-foreground">
        For Tauri apps, add tauri://localhost, http://tauri.localhost, or https://tauri.localhost to match each supported platform.
        These origins are shared by other Tauri apps; use server-signed identities to verify customers.
      </p>
      <div className="flex items-center gap-2">
        <Checkbox id="widget-allow-all-origins" checked={allowAll} disabled={mutation.isPending} onCheckedChange={value => { setAllowAll(value === true); setDirty(true); }} />
        <Label htmlFor="widget-allow-all-origins">Allow all origins</Label>
      </div>
      <p className="text-xs text-muted-foreground">Allows any website or desktop app to connect, including apps that send no origin. Session and identity verification settings still apply.</p>
      {!allowAll && !installation.allowed_origins?.length && <p role="status" className="text-sm text-quiet-accent">Add your website or desktop app origin to enable the widget.</p>}
      <div className="space-y-2">
        <Label htmlFor="widget-origins">Allowed origins</Label>
        <Textarea id="widget-origins" value={origins} placeholder={'https://www.example.com\nhttp://localhost:3000'} rows={3}
          disabled={allowAll || mutation.isPending} onChange={event => { setOrigins(event.target.value); setDirty(true); }} />
      </div>
      <div className="space-y-2">
        <Label htmlFor="widget-identity-mode">Visitor identity verification</Label>
        <Select value={mode} disabled={mutation.isPending} onValueChange={value => { setMode(value as OriginSettings['identity_verification_mode']); setDirty(true); }}>
          <SelectTrigger id="widget-identity-mode"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="report_only">Accept unverified identities</SelectItem>
            <SelectItem value="enforced">Require server-signed identities</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">Unverified identities remain visitor claims. Requiring signatures needs a server-side HMAC integration; anonymous chat still works. Never put the signing secret in your snippet.</p>
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <Button type="button" size="sm" disabled={!dirty || mutation.isPending} onClick={() => void save()}>{mutation.isPending ? 'Saving…' : 'Save origin settings'}</Button>
      <WidgetSigningSecret
        workspaceId={workspaceId}
        configured={installation.signing_secret_configured ?? true}
        canManage={canManageSigningSecret}
        enforced={installation.identity_verification_mode === 'enforced'}
      />
    </section>
  );
}
