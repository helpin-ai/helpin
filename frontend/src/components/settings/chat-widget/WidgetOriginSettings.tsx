import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useUpdateChatSettings } from '@/hooks/queries/useSupport';
import type { SupportInstallationResponse, WidgetOriginSettings as OriginSettings } from '@/lib/pmTypes';

export function parseWidgetOrigins(value: string): string[] {
  return [...new Set(value.split('\n').map(line => line.trim().replace(/\/$/, '')).filter(Boolean).map(origin => {
    const url = new URL(origin);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password ||
      url.pathname !== '/' || url.search || url.hash || /[?#*]/.test(origin) || origin.endsWith('/')) {
      throw new Error('Use a website origin such as https://www.example.com, without a path or wildcard.');
    }
    return url.origin;
  }))].sort();
}

export function WidgetOriginSettings({ workspaceId, installation }: { workspaceId: string; installation: SupportInstallationResponse }) {
  const [origins, setOrigins] = useState((installation.allowed_origins ?? []).join('\n'));
  const [mode, setMode] = useState<OriginSettings['identity_verification_mode']>(installation.identity_verification_mode);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState('');
  const mutation = useUpdateChatSettings(workspaceId);
  useEffect(() => {
    if (!dirty) {
      setOrigins((installation.allowed_origins ?? []).join('\n'));
      setMode(installation.identity_verification_mode);
    }
  }, [installation.allowed_origins, installation.identity_verification_mode, dirty]);
  async function save() {
    setError('');
    try {
      const allowedOrigins = parseWidgetOrigins(origins);
      if (mode === 'enforced' && !allowedOrigins.length) throw new Error('Add your website origin before requiring signed identities.');
      await mutation.mutateAsync({ allowed_origins: allowedOrigins, identity_verification_mode: mode });
      setDirty(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Website settings could not be saved.');
    }
  }
  return (
    <section className="space-y-4 border-b border-quiet-divider-strong px-4 py-6" aria-labelledby="widget-origin-title">
      <h3 id="widget-origin-title" className="text-sm font-medium">1. Add your website origin</h3>
      <p className="text-sm text-muted-foreground">
        Only these websites can connect to your widget. Include the scheme and any non-default port, one origin per line.
        Add local test and preview origins explicitly. An empty list blocks all visitor access.
      </p>
      {!installation.allowed_origins?.length && <p role="status" className="text-sm text-quiet-accent">Add your website origin to enable the widget.</p>}
      <div className="space-y-2">
        <Label htmlFor="widget-origins">Allowed website origins</Label>
        <Textarea id="widget-origins" value={origins} placeholder={'https://www.example.com\nhttp://localhost:3000'} rows={3}
          disabled={mutation.isPending} onChange={event => { setOrigins(event.target.value); setDirty(true); }} />
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
      <Button type="button" size="sm" disabled={!dirty || mutation.isPending} onClick={() => void save()}>{mutation.isPending ? 'Saving…' : 'Save website settings'}</Button>
    </section>
  );
}
