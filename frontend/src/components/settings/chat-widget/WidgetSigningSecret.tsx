import { useState } from 'react';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuietIconAction, QuietTextAction } from '@/components/design-system/quiet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Copy01Icon, ViewIcon, ViewOffIcon } from '@/lib/icons';
import { useRevealWidgetSigningSecret, useRotateWidgetSigningSecret } from '@/hooks/queries/useSupport';

const MASKED_SECRET = '•'.repeat(32);

interface WidgetSigningSecretProps {
  workspaceId: string;
  configured: boolean;
  canManage: boolean;
  enforced: boolean;
}

/**
 * Lets support admins view, copy, and regenerate the widget identity signing
 * secret. The secret stays in component state only; it is fetched on demand
 * through an audited endpoint and is never cached.
 */
export function WidgetSigningSecret({ workspaceId, configured, canManage, enforced }: WidgetSigningSecretProps) {
  const [secret, setSecret] = useState<string | null>(null);
  const [visible, setVisible] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [error, setError] = useState('');
  const reveal = useRevealWidgetSigningSecret(workspaceId);
  const rotate = useRotateWidgetSigningSecret(workspaceId);
  const busy = reveal.isPending || rotate.isPending;
  const hasSecret = configured || secret !== null;

  async function loadSecret(): Promise<string | null> {
    if (secret !== null) return secret;
    setError('');
    try {
      const { secret_key } = await reveal.mutateAsync();
      setSecret(secret_key);
      return secret_key;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'The signing secret could not be loaded.');
      return null;
    }
  }

  async function toggleVisible() {
    if (visible) { setVisible(false); return; }
    if (await loadSecret()) setVisible(true);
  }

  async function copy() {
    const value = await loadSecret();
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      toast.success('Signing secret copied');
    } catch {
      toast.error("Couldn't copy signing secret");
    }
  }

  async function regenerate() {
    setConfirmOpen(false);
    setError('');
    try {
      const { secret_key } = await rotate.mutateAsync();
      setSecret(secret_key);
      setVisible(true);
      toast.success(configured ? 'Signing secret regenerated' : 'Signing secret generated', {
        description: 'Update your server configuration with the new secret.',
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'The signing secret could not be regenerated.');
    }
  }

  return (
    <div className="space-y-2 border-t border-quiet-divider-light pt-4" aria-labelledby="widget-signing-secret-label">
      <Label id="widget-signing-secret-label" htmlFor="widget-signing-secret">Identity signing secret</Label>
      <p className="text-xs text-muted-foreground">
        Your server uses this secret to sign visitor identities with HMAC-SHA256. Keep it in your backend configuration only.
      </p>
      {!canManage ? (
        <p className="text-sm text-quiet-text-secondary">Only workspace admins can view or regenerate the signing secret.</p>
      ) : !hasSecret ? (
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-quiet-text-secondary">No signing secret has been issued yet.</p>
          <QuietTextAction type="button" disabled={busy} onClick={() => void regenerate()}>
            {rotate.isPending ? 'Generating…' : 'Generate secret'}
          </QuietTextAction>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-1">
            <Input
              id="widget-signing-secret"
              readOnly
              value={visible && secret ? secret : MASKED_SECRET}
              aria-label={visible ? 'Identity signing secret' : 'Identity signing secret (hidden)'}
              className="min-w-0 flex-1 font-mono text-xs"
              autoComplete="off"
              spellCheck={false}
            />
            <Tooltip>
              <TooltipTrigger asChild>
                <QuietIconAction type="button" disabled={busy} aria-label={visible ? 'Hide signing secret' : 'Reveal signing secret'} onClick={() => void toggleVisible()}>
                  {visible ? <ViewOffIcon className="h-4 w-4" /> : <ViewIcon className="h-4 w-4" />}
                </QuietIconAction>
              </TooltipTrigger>
              <TooltipContent>{visible ? 'Hide' : 'Reveal'}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <QuietIconAction type="button" disabled={busy} aria-label="Copy signing secret" onClick={() => void copy()}>
                  <Copy01Icon className="h-4 w-4" />
                </QuietIconAction>
              </TooltipTrigger>
              <TooltipContent>Copy</TooltipContent>
            </Tooltip>
          </div>
          <QuietTextAction type="button" disabled={busy} onClick={() => setConfirmOpen(true)}>
            {rotate.isPending ? 'Regenerating…' : 'Regenerate secret'}
          </QuietTextAction>
        </>
      )}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Regenerate signing secret?"
        description={
          `The current secret stops working immediately. Until you update your server with the new secret, ${enforced
            ? 'identities signed with the old secret are rejected and those visitors can only chat anonymously'
            : 'identities signed with the old secret are treated as unverified'}. The public widget key does not change.`
        }
        confirmLabel="Regenerate secret"
        variant="destructive"
        onConfirm={() => void regenerate()}
      />
    </div>
  );
}
