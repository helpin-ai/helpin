import { useEffect, useState, type FormEvent } from 'react';
import { format } from 'date-fns';
import QRCode from 'qrcode';
import { useTitle } from '@/hooks/useTitle';
import { persistAuthSession, useAuthStore } from '@/stores/authStore';
import { authService } from '@/lib/services/authService';
import { passkeyService } from '@/lib/services/passkeyService';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  ArrowLeft02Icon,
  ArrowReloadHorizontalIcon,
  CheckmarkCircle02Icon,
  Copy01Icon,
  Delete01Icon,
  Download04Icon,
  Key01Icon,
  LaptopIcon,
  Loading01Icon,
  LockKeyIcon,
  Shield02Icon,
  SmartPhone01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';
import type { Passkey, RecoveryCodesResponse, TwoFASetupResponse } from '@/lib/types';
import { QuietPageHeader } from '@/components/design-system/quiet';

type ManualSetupDetails = {
  accountName: string;
  issuer: string;
  secret: string;
};

function RecoveryCodeList({ codes }: { codes: string[] }) {
  return (
    <div className="grid grid-cols-2 gap-2">
      {codes.map((code) => (
        <div key={code} className="rounded-md border bg-muted/30 px-3 py-2 font-mono text-sm tracking-[0.18em]">
          {code}
        </div>
      ))}
    </div>
  );
}

function parseManualSetupDetails(provisioningURI?: string | null): ManualSetupDetails | null {
  if (!provisioningURI) {
    return null;
  }

  try {
    const parsed = new URL(provisioningURI);
    const accountLabel = decodeURIComponent(parsed.pathname.replace(/^\/+/, ''));
    const accountName = accountLabel.includes(':') ? accountLabel.slice(accountLabel.indexOf(':') + 1) : accountLabel;
    const issuer = parsed.searchParams.get('issuer') ?? '';
    const secret = parsed.searchParams.get('secret') ?? '';
    if (!accountName || !issuer || !secret) {
      return null;
    }

    return { accountName, issuer, secret };
  } catch {
    return null;
  }
}

function formatManualSecret(secret: string) {
  return secret.match(/.{1,4}/g)?.join(' ') ?? secret;
}

export default function SecuritySettings() {
  useTitle('Security');
  const { user } = useAuthStore();

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);

  const [twoFAEnabled, setTwoFAEnabled] = useState(user?.two_fa_enabled ?? false);
  const [loadingTwoFAStatus, setLoadingTwoFAStatus] = useState(true);
  const [setupDialogOpen, setSetupDialogOpen] = useState(false);
  const [setupPassword, setSetupPassword] = useState('');
  const [setupProvisioning, setSetupProvisioning] = useState<TwoFASetupResponse | null>(null);
  const [setupQRCodeUrl, setSetupQRCodeUrl] = useState('');
  const [setupQRCodeStatus, setSetupQRCodeStatus] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [setupVerificationCode, setSetupVerificationCode] = useState('');
  const [setupSubmitting, setSetupSubmitting] = useState(false);
  const [setupStep, setSetupStep] = useState<'password' | 'recovery' | 'verify'>('password');
  const [recoveryAcknowledged, setRecoveryAcknowledged] = useState(false);
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [disablePassword, setDisablePassword] = useState('');
  const [disableSubmitting, setDisableSubmitting] = useState(false);
  const [recoveryDialogOpen, setRecoveryDialogOpen] = useState(false);
  const [recoveryPassword, setRecoveryPassword] = useState('');
  const [recoveryVerificationCode, setRecoveryVerificationCode] = useState('');
  const [regeneratedRecoveryCodes, setRegeneratedRecoveryCodes] = useState<RecoveryCodesResponse | null>(null);
  const [recoverySubmitting, setRecoverySubmitting] = useState(false);

  const [passkeys, setPasskeys] = useState<Passkey[]>([]);
  const [loadingPasskeys, setLoadingPasskeys] = useState(true);
  const [addingPasskey, setAddingPasskey] = useState(false);
  const [deletingPasskeyId, setDeletingPasskeyId] = useState<string | null>(null);
  const manualSetupDetails = parseManualSetupDetails(setupProvisioning?.provisioning_uri);
  const passkeySupported = passkeyService.isSupported();

  useEffect(() => {
    let cancelled = false;

    const loadTwoFAStatus = async () => {
      setLoadingTwoFAStatus(true);
      const { data, error } = await authService.get2FAStatus();
      if (cancelled) return;

      if (error || !data) {
        setLoadingTwoFAStatus(false);
        toast.error(error ?? 'Failed to load two-factor authentication status');
        return;
      }

      setTwoFAEnabled(data.enabled);
      setLoadingTwoFAStatus(false);
    };

    void loadTwoFAStatus();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    const loadPasskeys = async () => {
      setLoadingPasskeys(true);
      const { data, error } = await passkeyService.listPasskeys();
      if (cancelled) return;

      if (error || !data) {
        setLoadingPasskeys(false);
        toast.error(error ?? 'Failed to load passkeys');
        return;
      }

      setPasskeys(data);
      setLoadingPasskeys(false);
    };

    void loadPasskeys();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    const buildQRCode = async () => {
      if (!setupProvisioning?.provisioning_uri) {
        setSetupQRCodeUrl('');
        setSetupQRCodeStatus('idle');
        return;
      }

      setSetupQRCodeStatus('loading');
      try {
        const dataUrl = await QRCode.toDataURL(setupProvisioning.provisioning_uri, {
          margin: 1,
          width: 256,
        });
        if (!cancelled) {
          setSetupQRCodeUrl(dataUrl);
          setSetupQRCodeStatus('ready');
        }
      } catch {
        if (!cancelled) {
          setSetupQRCodeUrl('');
          setSetupQRCodeStatus('error');
        }
      }
    };

    void buildQRCode();
    return () => {
      cancelled = true;
    };
  }, [setupProvisioning?.provisioning_uri]);

  const syncTwoFAState = (enabled: boolean) => {
    setTwoFAEnabled(enabled);
    const existingUser = useAuthStore.getState().user;
    if (existingUser) {
      useAuthStore.setState({
        user: {
          ...existingUser,
          two_fa_enabled: enabled,
        },
      });
    }
  };

  const copyText = async (value: string, successMessage: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(successMessage);
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  const copyRecoveryCodes = async (codes: string[]) => copyText(codes.join('\n'), 'Recovery codes copied');

  const resetSetupDialog = () => {
    setSetupDialogOpen(false);
    setSetupPassword('');
    setSetupProvisioning(null);
    setSetupQRCodeUrl('');
    setSetupQRCodeStatus('idle');
    setSetupVerificationCode('');
    setSetupStep('password');
    setRecoveryAcknowledged(false);
  };

  const downloadRecoveryCodes = (codes: string[]) => {
    const header = `Helpin recovery codes for ${user?.email ?? 'your account'}\nGenerated ${new Date().toLocaleString()}\n\nEach code can be used once to sign in if you lose access to your authenticator app.\nStore these somewhere safe — they will not be shown again.\n\n`;
    const blob = new Blob([header + codes.join('\n') + '\n'], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'helpin-recovery-codes.txt';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const resetRecoveryDialog = () => {
    setRecoveryDialogOpen(false);
    setRecoveryPassword('');
    setRecoveryVerificationCode('');
    setRegeneratedRecoveryCodes(null);
  };

  const handleChangePassword = async (e: FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      toast.error('New password must be at least 8 characters');
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error('Passwords do not match');
      return;
    }
    setChangingPassword(true);
    const { error } = await authService.changePassword(currentPassword, newPassword);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Password updated successfully');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    }
    setChangingPassword(false);
  };

  const handleSetupTwoFA = async (e: FormEvent) => {
    e.preventDefault();
    setSetupSubmitting(true);
    const { data, error } = await authService.setup2FA(setupPassword);
    setSetupSubmitting(false);

    if (error || !data) {
      toast.error(error ?? 'Failed to start two-factor setup');
      return;
    }

    setSetupProvisioning(data);
    setSetupVerificationCode('');
    setRecoveryAcknowledged(false);
    setSetupStep('recovery');
  };

  const handleVerifyTwoFASetup = async (e: FormEvent) => {
    e.preventDefault();
    setSetupSubmitting(true);
    const { data, error } = await authService.verify2FASetup(setupVerificationCode);
    setSetupSubmitting(false);

    if (error) {
      toast.error(error);
      return;
    }

    syncTwoFAState(true);
    if (data) {
      persistAuthSession(data.user, data.access_token, data.refresh_token, localStorage.getItem('remember_me') === '1');
    }
    toast.success('Two-factor authentication enabled');
    resetSetupDialog();
  };

  const handleDisableTwoFA = async (e: FormEvent) => {
    e.preventDefault();
    setDisableSubmitting(true);
    const { error } = await authService.disable2FA(disablePassword);
    setDisableSubmitting(false);

    if (error) {
      toast.error(error);
      return;
    }

    syncTwoFAState(false);
    toast.success('Two-factor authentication disabled');
    setDisablePassword('');
    setDisableDialogOpen(false);
  };

  const handleRegenerateRecoveryCodes = async (e: FormEvent) => {
    e.preventDefault();
    setRecoverySubmitting(true);
    const { data, error } = await authService.regenerateRecoveryCodes(recoveryPassword, recoveryVerificationCode);
    setRecoverySubmitting(false);

    if (error || !data) {
      toast.error(error ?? 'Failed to regenerate recovery codes');
      return;
    }

    setRegeneratedRecoveryCodes(data);
    toast.success('Recovery codes regenerated');
  };

  const handleAddPasskey = async () => {
    setAddingPasskey(true);
    const { data, error } = await passkeyService.beginRegistration();
    setAddingPasskey(false);

    if (error || !data) {
      toast.error(error ?? 'Failed to add passkey');
      return;
    }

    setPasskeys((current) => [data, ...current.filter((passkey) => passkey.id !== data.id)]);
    toast.success('Passkey added');
  };

  const handleDeletePasskey = async (id: string) => {
    setDeletingPasskeyId(id);
    const { error } = await passkeyService.deletePasskey(id);
    setDeletingPasskeyId(null);

    if (error) {
      toast.error(error);
      return;
    }

    setPasskeys((current) => current.filter((passkey) => passkey.id !== id));
    toast.success('Passkey removed');
  };

  return (
    <div className="space-y-4">
      <Dialog
        open={setupDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            resetSetupDialog();
            return;
          }
          setSetupDialogOpen(open);
        }}
      >
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Enable Two-Factor Authentication</DialogTitle>
            <DialogDescription>
              Protect your Helpin account with a time-based code from Google Authenticator, Authy, 1Password, or another compatible app.
            </DialogDescription>
          </DialogHeader>

          {setupProvisioning && setupStep !== 'password' && (
            <div className="flex items-center gap-3 rounded-lg border bg-muted/30 p-3 text-sm">
              <div className={`flex items-center gap-2 ${setupStep === 'recovery' ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>
                {setupStep === 'verify' ? (
                  <CheckmarkCircle02Icon className="h-5 w-5 text-emerald-600" />
                ) : (
                  <span className="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">1</span>
                )}
                Save recovery codes
              </div>
              <div className="h-px flex-1 bg-border" />
              <div className={`flex items-center gap-2 ${setupStep === 'verify' ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>
                <span className={`flex h-5 w-5 items-center justify-center rounded-full text-xs font-semibold ${setupStep === 'verify' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>2</span>
                Connect authenticator
              </div>
            </div>
          )}

          {setupStep === 'password' && (
            <form onSubmit={handleSetupTwoFA} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="setup-2fa-password">Current Password</Label>
                <Input
                  id="setup-2fa-password"
                  type="password"
                  value={setupPassword}
                  onChange={(e) => setSetupPassword(e.target.value)}
                  placeholder="Confirm your password"
                  required
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={resetSetupDialog}>
                  Cancel
                </Button>
                <Button type="submit" disabled={setupSubmitting}>
                  {setupSubmitting ? 'Preparing...' : 'Continue'}
                </Button>
              </DialogFooter>
            </form>
          )}

          {setupProvisioning && setupStep === 'recovery' && (
            <div className="space-y-5">
              <div className="space-y-1">
                <h3 className="text-base font-semibold">Save your recovery codes</h3>
                <p className="text-sm text-muted-foreground">
                  These single-use codes let you sign in if you lose access to your authenticator app. Store them in a password manager or print them — they won&apos;t be shown again.
                </p>
              </div>
              <div className="space-y-3 rounded-lg border p-4">
                <div className="flex items-center justify-between">
                  <Label>Recovery Codes</Label>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => void copyRecoveryCodes(setupProvisioning.recovery_codes)}
                    >
                      <Copy01Icon className="mr-2 h-4 w-4" />
                      Copy all
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => downloadRecoveryCodes(setupProvisioning.recovery_codes)}
                    >
                      <Download04Icon className="mr-2 h-4 w-4" />
                      Download .txt
                    </Button>
                  </div>
                </div>
                <RecoveryCodeList codes={setupProvisioning.recovery_codes} />
              </div>
              <label className="flex items-start gap-3 rounded-lg border bg-muted/20 p-3 text-sm">
                <Checkbox
                  id="recovery-acknowledged"
                  checked={recoveryAcknowledged}
                  onCheckedChange={(checked) => setRecoveryAcknowledged(checked === true)}
                  className="mt-0.5"
                />
                <span className="leading-snug">
                  I&apos;ve saved my recovery codes somewhere safe. I understand they won&apos;t be shown again and I&apos;ll need them if I lose my authenticator.
                </span>
              </label>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={resetSetupDialog}>
                  Cancel
                </Button>
                <Button
                  type="button"
                  disabled={!recoveryAcknowledged}
                  onClick={() => setSetupStep('verify')}
                >
                  Continue
                </Button>
              </DialogFooter>
            </div>
          )}

          {setupProvisioning && setupStep === 'verify' && (
            <div className="space-y-5">
              <div className="space-y-1">
                <h3 className="text-base font-semibold">Connect your authenticator app</h3>
                <p className="text-sm text-muted-foreground">
                  Scan the QR code with Google Authenticator, Authy, 1Password, or any TOTP app, then enter the 6-digit code it shows.
                </p>
              </div>
              <div className="flex justify-center rounded-xl border bg-white p-4">
                {setupQRCodeStatus === 'ready' && setupQRCodeUrl ? (
                  <img src={setupQRCodeUrl} alt="Authenticator QR code" className="h-48 w-48" />
                ) : setupQRCodeStatus === 'error' ? (
                  <div className="flex h-48 w-48 flex-col items-center justify-center gap-2 px-4 text-center text-sm text-muted-foreground">
                    <p>QR code unavailable.</p>
                    <p>Use the manual setup key below.</p>
                  </div>
                ) : (
                  <div className="flex h-48 w-48 items-center justify-center text-sm text-muted-foreground">
                    Generating QR code...
                  </div>
                )}
              </div>
              {manualSetupDetails && (
                <div className="space-y-3 rounded-lg border p-4">
                  <div className="space-y-1">
                    <Label>Can&apos;t scan the QR code?</Label>
                    <p className="text-sm text-muted-foreground">
                      Add a TOTP account manually with the setup key below.
                    </p>
                  </div>
                  <div className="rounded-md border bg-muted/30 px-3 py-2">
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">Setup Key</div>
                    <div className="mt-1 break-all font-mono text-sm tracking-[0.18em]">
                      {formatManualSecret(manualSetupDetails.secret)}
                    </div>
                  </div>
                  <div className="grid gap-2 text-sm text-muted-foreground sm:grid-cols-2">
                    <div className="rounded-md border bg-muted/30 px-3 py-2">
                      <div className="text-xs uppercase tracking-[0.18em]">Account</div>
                      <div className="mt-1 text-foreground">{manualSetupDetails.accountName}</div>
                    </div>
                    <div className="rounded-md border bg-muted/30 px-3 py-2">
                      <div className="text-xs uppercase tracking-[0.18em]">Issuer</div>
                      <div className="mt-1 text-foreground">{manualSetupDetails.issuer}</div>
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => void copyText(manualSetupDetails.secret, 'Setup key copied')}
                    >
                      <Copy01Icon className="mr-2 h-4 w-4" />
                      Copy setup key
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => void copyText(setupProvisioning.provisioning_uri, 'Provisioning link copied')}
                    >
                      <Copy01Icon className="mr-2 h-4 w-4" />
                      Copy provisioning link
                    </Button>
                  </div>
                </div>
              )}

              <form onSubmit={handleVerifyTwoFASetup} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="setup-2fa-code">Authenticator Code</Label>
                  <Input
                    id="setup-2fa-code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    placeholder="123456"
                    value={setupVerificationCode}
                    onChange={(e) => setSetupVerificationCode(e.target.value)}
                    required
                  />
                </div>
                <DialogFooter>
                  <Button type="button" variant="ghost" onClick={() => setSetupStep('recovery')}>
                    <ArrowLeft02Icon className="mr-2 h-4 w-4" />
                    Back
                  </Button>
                  <Button type="button" variant="outline" onClick={resetSetupDialog}>
                    Cancel
                  </Button>
                  <Button type="submit" disabled={setupSubmitting}>
                    {setupSubmitting ? 'Verifying...' : 'Enable 2FA'}
                  </Button>
                </DialogFooter>
              </form>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={disableDialogOpen}
        onOpenChange={(open) => {
          setDisableDialogOpen(open);
          if (!open) {
            setDisablePassword('');
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Disable Two-Factor Authentication</DialogTitle>
            <DialogDescription>
              Enter your current password to remove the authenticator requirement from future logins.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleDisableTwoFA} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="disable-2fa-password">Current Password</Label>
              <Input
                id="disable-2fa-password"
                type="password"
                value={disablePassword}
                onChange={(e) => setDisablePassword(e.target.value)}
                placeholder="Confirm your password"
                required
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDisableDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" variant="destructive" disabled={disableSubmitting}>
                {disableSubmitting ? 'Disabling...' : 'Disable 2FA'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={recoveryDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            resetRecoveryDialog();
            return;
          }
          setRecoveryDialogOpen(open);
        }}
      >
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Regenerate Recovery Codes</DialogTitle>
            <DialogDescription>
              Generate a new set of single-use recovery codes. Your previous codes will stop working immediately.
            </DialogDescription>
          </DialogHeader>

          {regeneratedRecoveryCodes ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label>New Recovery Codes</Label>
                <Button type="button" variant="outline" size="sm" onClick={() => void copyRecoveryCodes(regeneratedRecoveryCodes.recovery_codes)}>
                  <Copy01Icon className="mr-2 h-4 w-4" />
                  Copy all
                </Button>
              </div>
              <RecoveryCodeList codes={regeneratedRecoveryCodes.recovery_codes} />
              <DialogFooter>
                <Button type="button" onClick={resetRecoveryDialog}>
                  Done
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <form onSubmit={handleRegenerateRecoveryCodes} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="recovery-password">Current Password</Label>
                <Input
                  id="recovery-password"
                  type="password"
                  value={recoveryPassword}
                  onChange={(e) => setRecoveryPassword(e.target.value)}
                  placeholder="Confirm your password"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="recovery-code-verify">Authenticator Code</Label>
                <Input
                  id="recovery-code-verify"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  value={recoveryVerificationCode}
                  onChange={(e) => setRecoveryVerificationCode(e.target.value)}
                  placeholder="123456"
                  required
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={resetRecoveryDialog}>
                  Cancel
                </Button>
                <Button type="submit" disabled={recoverySubmitting}>
                  {recoverySubmitting ? 'Regenerating...' : 'Regenerate Codes'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <QuietPageHeader title="Security" />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <LockKeyIcon className="h-4 w-4 text-muted-foreground" />
            Password
          </CardTitle>
          <CardDescription>Update your account password.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="current-password">Current password</Label>
                <Input
                  id="current-password"
                  type="password"
                  value={currentPassword}
                  onChange={e => setCurrentPassword(e.target.value)}
                  placeholder="Enter current password"
                  required
                />
              </div>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="new-password">New password</Label>
                <Input
                  id="new-password"
                  type="password"
                  value={newPassword}
                  onChange={e => setNewPassword(e.target.value)}
                  placeholder="At least 8 characters"
                  required
                  minLength={8}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirm-password">Confirm new password</Label>
                <Input
                  id="confirm-password"
                  type="password"
                  value={confirmPassword}
                  onChange={e => setConfirmPassword(e.target.value)}
                  placeholder="Re-enter new password"
                  required
                  minLength={8}
                />
              </div>
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={changingPassword}>
                {changingPassword ? 'Updating...' : 'Update password'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <CardTitle className="flex items-center gap-2 text-base">
                <Shield02Icon className="h-4 w-4 text-muted-foreground" />
                Two-factor authentication
              </CardTitle>
              <CardDescription>
                Add an authenticator app check to your Helpin sign-ins and keep recovery codes for backup access.
              </CardDescription>
            </div>
            <StatusPill loading={loadingTwoFAStatus} active={twoFAEnabled} />
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {twoFAEnabled ? (
            <>
              <div className="flex gap-3 rounded-lg border bg-muted/30 p-4 text-sm">
                <SmartPhone01Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="space-y-1">
                  <p className="font-medium text-foreground">Authenticator app required at login</p>
                  <p className="text-muted-foreground">
                    Sign-ins now require your password plus a time-based one-time code. Recovery codes are single-use and should be stored offline.
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={() => setRecoveryDialogOpen(true)}>
                  <ArrowReloadHorizontalIcon className="mr-2 h-4 w-4" />
                  Regenerate recovery codes
                </Button>
                <Button type="button" variant="destructive" onClick={() => setDisableDialogOpen(true)}>
                  <LockKeyIcon className="mr-2 h-4 w-4" />
                  Disable 2FA
                </Button>
              </div>
            </>
          ) : (
            <>
              <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                Two-factor authentication is currently off. Enable it to require a code from your authenticator app whenever you sign in.
              </div>
              <Button type="button" onClick={() => setSetupDialogOpen(true)}>
                <Shield02Icon className="mr-2 h-4 w-4" />
                Enable 2FA
              </Button>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <CardTitle className="flex items-center gap-2 text-base">
                <Key01Icon className="h-4 w-4 text-muted-foreground" />
                Passkeys
              </CardTitle>
              <CardDescription>
                Sign in with Face ID, Touch ID, Windows Hello, or another device-backed passkey. Verified passkeys satisfy 2FA.
              </CardDescription>
            </div>
            <Button type="button" size="sm" onClick={() => void handleAddPasskey()} disabled={addingPasskey || !passkeySupported}>
              {addingPasskey ? 'Adding...' : 'Add a passkey'}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {!passkeySupported && (
            <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
              This browser can manage your existing passkeys here, but it cannot create new passkeys.
            </div>
          )}
          {loadingPasskeys ? (
            <div className="rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground">
              Loading passkeys...
            </div>
          ) : passkeys.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed py-8 text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <Key01Icon className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium">No passkeys yet</p>
              <p className="max-w-sm text-xs text-muted-foreground">
                Add one to sign in with a single click on supported devices.
              </p>
            </div>
          ) : (
            <div className="divide-y rounded-lg border">
              {passkeys.map((passkey) => (
                <div key={passkey.id} className="flex items-center justify-between gap-4 p-4">
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border bg-muted/40">
                      <LaptopIcon className="h-4 w-4 text-muted-foreground" />
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-medium">{passkey.name}</span>
                        {passkey.verified && (
                          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400">
                            <CheckmarkCircle02Icon className="h-3 w-3" />
                            Verified
                          </span>
                        )}
                      </div>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        Added {format(new Date(passkey.created_at), 'MMM d, yyyy')}
                      </p>
                    </div>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={deletingPasskeyId === passkey.id}
                    onClick={() => void handleDeletePasskey(passkey.id)}
                  >
                    <Delete01Icon className="mr-2 h-4 w-4" />
                    {deletingPasskeyId === passkey.id ? 'Removing...' : 'Delete'}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatusPill({ loading, active }: { loading: boolean; active: boolean }) {
  if (loading) {
    return (
      <span className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground">
        <Loading01Icon className="h-3 w-3 animate-spin" />
        Checking
      </span>
    );
  }
  return (
    <span
      className={
        active
          ? 'inline-flex shrink-0 items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400'
          : 'inline-flex shrink-0 items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground'
      }
    >
      <span
        className={
          active
            ? 'h-1.5 w-1.5 rounded-full bg-emerald-500'
            : 'h-1.5 w-1.5 rounded-full bg-muted-foreground/50'
        }
      />
      {active ? 'Enabled' : 'Disabled'}
    </span>
  );
}
