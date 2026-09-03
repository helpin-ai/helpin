import { useState, type FormEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import QRCode from 'qrcode';
import { toast } from 'sonner';
import { authService } from '@/lib/services/authService';
import { persistAuthSession } from '@/stores/authStore';
import type { TwoFASetupResponse } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Copy01Icon, Shield02Icon } from '@/lib/icons';

type MFARequiredGateProps = {
  workspaceName: string;
  mfaEnabled: boolean;
  onComplete: () => void;
};

function persistReturnedSession(data: { user: Parameters<typeof persistAuthSession>[0]; access_token: string; refresh_token: string }) {
  persistAuthSession(data.user, data.access_token, data.refresh_token, localStorage.getItem('remember_me') === '1');
}

function RecoveryCodes({ codes }: { codes: string[] }) {
  return (
    <div className="grid grid-cols-2 gap-2">
      {codes.map((code) => (
        <div key={code} className="rounded-md border bg-muted/30 px-3 py-2 font-mono text-sm tracking-[0.16em]">
          {code}
        </div>
      ))}
    </div>
  );
}

export function MFARequiredGate({ workspaceName, mfaEnabled, onComplete }: MFARequiredGateProps) {
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [useRecoveryCode, setUseRecoveryCode] = useState(false);
  const [setup, setSetup] = useState<TwoFASetupResponse | null>(null);
  const [savedRecoveryCodes, setSavedRecoveryCodes] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const provisioningUri = setup?.provisioning_uri ?? '';
  const qrCodeQuery = useQuery({
    queryKey: ['two-fa-setup-qr', provisioningUri, 224],
    queryFn: () => QRCode.toDataURL(provisioningUri, { margin: 1, width: 224 }),
    enabled: !!provisioningUri,
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });
  const qrCodeUrl = provisioningUri ? (qrCodeQuery.data ?? '') : '';

  const handleStepUp = async (event: FormEvent) => {
    event.preventDefault();
    if (!code.trim()) return;
    setSubmitting(true);
    const { data, error } = await authService.stepUp2FA(code, useRecoveryCode);
    setSubmitting(false);
    if (error || !data) {
      toast.error(error ?? 'Verification failed');
      return;
    }
    persistReturnedSession(data);
    toast.success('Two-factor authentication verified');
    onComplete();
  };

  const handleStartSetup = async (event: FormEvent) => {
    event.preventDefault();
    if (!password.trim()) return;
    setSubmitting(true);
    const { data, error } = await authService.setup2FA(password);
    setSubmitting(false);
    if (error || !data) {
      toast.error(error ?? 'Failed to start two-factor setup');
      return;
    }
    setSetup(data);
    setCode('');
  };

  const handleFinishSetup = async (event: FormEvent) => {
    event.preventDefault();
    if (!code.trim() || !savedRecoveryCodes) return;
    setSubmitting(true);
    const { data, error } = await authService.verify2FASetup(code);
    setSubmitting(false);
    if (error || !data) {
      toast.error(error ?? 'Failed to enable two-factor authentication');
      return;
    }
    persistReturnedSession(data);
    toast.success('Two-factor authentication enabled');
    onComplete();
  };

  const copyRecoveryCodes = async () => {
    if (!setup?.recovery_codes.length) return;
    try {
      await navigator.clipboard.writeText(setup.recovery_codes.join('\n'));
      toast.success('Recovery codes copied');
    } catch {
      toast.error('Failed to copy recovery codes');
    }
  };

  return (
    <div className="flex min-h-svh items-center justify-center bg-background px-4">
      <Card className="w-full max-w-[520px]">
        <CardHeader className="space-y-2">
          <div className="flex items-center gap-2">
            <Shield02Icon className="h-5 w-5 text-muted-foreground" />
            <CardTitle className="text-lg">Two-factor authentication required</CardTitle>
          </div>
          <CardDescription>
            {workspaceName} requires two-factor authentication before you can continue.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {mfaEnabled ? (
            <form onSubmit={handleStepUp} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="mfa-code">{useRecoveryCode ? 'Recovery code' : 'Authenticator code'}</Label>
                <Input
                  id="mfa-code"
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  inputMode={useRecoveryCode ? 'text' : 'numeric'}
                  autoComplete="one-time-code"
                  placeholder={useRecoveryCode ? 'ABCD1234' : '123456'}
                />
              </div>
              <div className="flex items-center justify-between gap-3">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="px-0 text-muted-foreground"
                  onClick={() => {
                    setUseRecoveryCode((value) => !value);
                    setCode('');
                  }}
                >
                  {useRecoveryCode ? 'Use authenticator code' : 'Use recovery code'}
                </Button>
                <Button type="submit" disabled={submitting || !code.trim()}>
                  {submitting ? 'Verifying...' : 'Continue'}
                </Button>
              </div>
            </form>
          ) : setup ? (
            <form onSubmit={handleFinishSetup} className="space-y-5">
              <div className="flex flex-col gap-4 sm:flex-row">
                {qrCodeUrl ? (
                  <img src={qrCodeUrl} alt="Authenticator QR code" className="h-56 w-56 rounded-md border bg-white p-2" />
                ) : null}
                <div className="min-w-0 flex-1 space-y-2">
                  <Label>Recovery codes</Label>
                  <RecoveryCodes codes={setup.recovery_codes} />
                  <Button type="button" variant="outline" size="sm" onClick={() => void copyRecoveryCodes()}>
                    <Copy01Icon className="mr-2 h-4 w-4" />
                    Copy codes
                  </Button>
                </div>
              </div>
              <Separator />
              <div className="flex items-center gap-2">
                <Checkbox
                  id="saved-recovery-codes"
                  checked={savedRecoveryCodes}
                  onCheckedChange={(checked) => setSavedRecoveryCodes(checked === true)}
                />
                <Label htmlFor="saved-recovery-codes" className="text-sm font-normal">
                  I saved these recovery codes.
                </Label>
              </div>
              <div className="space-y-2">
                <Label htmlFor="setup-mfa-code">Authenticator code</Label>
                <Input
                  id="setup-mfa-code"
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder="123456"
                />
              </div>
              <div className="flex justify-end">
                <Button type="submit" disabled={submitting || !code.trim() || !savedRecoveryCodes}>
                  {submitting ? 'Enabling...' : 'Enable and continue'}
                </Button>
              </div>
            </form>
          ) : (
            <form onSubmit={handleStartSetup} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="mfa-password">Current password</Label>
                <Input
                  id="mfa-password"
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete="current-password"
                />
              </div>
              <div className="flex justify-end">
                <Button type="submit" disabled={submitting || !password.trim()}>
                  {submitting ? 'Preparing...' : 'Set up 2FA'}
                </Button>
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
