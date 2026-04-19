import { useEffect, useState, type FormEvent } from 'react';
import QRCode from 'qrcode';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { authService } from '@/lib/services/authService';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Separator } from '@/components/ui/separator';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  TEAM_MEMBER_AVATAR_STYLES,
  TEAM_MEMBER_AVATAR_BACKGROUND_COLORS,
  type TeamMemberAvatarBackgroundMode,
  type TeamMemberAvatarStyle,
  createTeamMemberAvatarSeed,
  hasGeneratedTeamMemberAvatar,
  normalizeTeamMemberAvatarBackgroundColor,
  normalizeTeamMemberAvatarBackgroundMode,
  normalizeTeamMemberAvatarStyle,
  resolveTeamMemberAvatarSrc,
} from '@/lib/teamMemberAvatar';
import { getInitials } from '@/lib/utils';
import {
  ArrowReloadHorizontalIcon,
  Camera01Icon,
  Copy01Icon,
  Loading01Icon,
  LockKeyIcon,
  Mail01Icon,
  Shield02Icon,
  SmartPhone01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { AvatarCropDialog } from '@/components/profile/AvatarCropDialog';
import { AvatarPickerDialog } from '@/components/profile/AvatarPickerDialog';
import { queryClient } from '@/lib/queryClient';
import { queryKeys } from '@/lib/queryKeys';
import type { RecoveryCodesResponse, TwoFASetupResponse } from '@/lib/types';

type PendingAvatarFile = {
  file: File;
  previewUrl: string;
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

export default function Profile() {
  useTitle('Profile');
  const { user, updateUser } = useAuthStore();
  const currentWorkspace = useWorkspaceStore((s) => s.currentWorkspace);
  const defaultAvatarStyle: TeamMemberAvatarStyle = TEAM_MEMBER_AVATAR_STYLES[0].value;
  const persistedGeneratedAvatarEnabled = hasGeneratedTeamMemberAvatar(user?.avatar_style, user?.avatar_seed);
  const persistedAvatarStyle = normalizeTeamMemberAvatarStyle(user?.avatar_style) ?? defaultAvatarStyle;
  const persistedAvatarSeed = user?.avatar_seed ?? '';
  const persistedAvatarBackgroundMode = normalizeTeamMemberAvatarBackgroundMode(user?.avatar_background_mode);
  const persistedAvatarBackgroundColor = normalizeTeamMemberAvatarBackgroundColor(user?.avatar_background_color) ?? TEAM_MEMBER_AVATAR_BACKGROUND_COLORS[0];
  const [fullName, setFullName] = useState(user?.full_name ?? '');
  const [saving, setSaving] = useState(false);
  const [generatedAvatarEnabled, setGeneratedAvatarEnabled] = useState(persistedGeneratedAvatarEnabled);
  const [avatarStyle, setAvatarStyle] = useState<TeamMemberAvatarStyle>(persistedAvatarStyle);
  const [avatarSeed, setAvatarSeed] = useState(persistedAvatarSeed || createTeamMemberAvatarSeed());
  const [avatarBackgroundMode, setAvatarBackgroundMode] = useState<TeamMemberAvatarBackgroundMode>(persistedAvatarBackgroundMode);
  const [avatarBackgroundColor, setAvatarBackgroundColor] = useState(persistedAvatarBackgroundColor);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [pendingAvatar, setPendingAvatar] = useState<PendingAvatarFile | null>(null);
  const [avatarDialogOpen, setAvatarDialogOpen] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [twoFAEnabled, setTwoFAEnabled] = useState(user?.two_fa_enabled ?? false);
  const [loadingTwoFAStatus, setLoadingTwoFAStatus] = useState(true);
  const [setupDialogOpen, setSetupDialogOpen] = useState(false);
  const [setupPassword, setSetupPassword] = useState('');
  const [setupProvisioning, setSetupProvisioning] = useState<TwoFASetupResponse | null>(null);
  const [setupQRCodeUrl, setSetupQRCodeUrl] = useState('');
  const [setupVerificationCode, setSetupVerificationCode] = useState('');
  const [setupSubmitting, setSetupSubmitting] = useState(false);
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [disablePassword, setDisablePassword] = useState('');
  const [disableSubmitting, setDisableSubmitting] = useState(false);
  const [recoveryDialogOpen, setRecoveryDialogOpen] = useState(false);
  const [recoveryPassword, setRecoveryPassword] = useState('');
  const [recoveryVerificationCode, setRecoveryVerificationCode] = useState('');
  const [regeneratedRecoveryCodes, setRegeneratedRecoveryCodes] = useState<RecoveryCodesResponse | null>(null);
  const [recoverySubmitting, setRecoverySubmitting] = useState(false);

  const initials = getInitials(user?.full_name || user?.email);
  const profileAvatarSrc = resolveTeamMemberAvatarSrc({
    avatarUrl: user?.avatar_url,
    avatarStyle: generatedAvatarEnabled ? avatarStyle : undefined,
    avatarSeed: generatedAvatarEnabled ? avatarSeed : undefined,
    avatarBackgroundMode: generatedAvatarEnabled ? avatarBackgroundMode : undefined,
    avatarBackgroundColor: generatedAvatarEnabled ? avatarBackgroundColor : undefined,
    fallbackSeed: user?.full_name ?? user?.email,
  });
  const shouldPersistAvatarBackgroundColor = generatedAvatarEnabled && avatarStyle === 'personas' && avatarBackgroundMode === 'color';
  const persistedShouldPersistAvatarBackgroundColor = persistedGeneratedAvatarEnabled
    && persistedAvatarStyle === 'personas'
    && persistedAvatarBackgroundMode === 'color';
  const hasProfileChanges = fullName !== (user?.full_name ?? '')
    || generatedAvatarEnabled !== persistedGeneratedAvatarEnabled
    || (generatedAvatarEnabled && (
      avatarStyle !== persistedAvatarStyle
      || avatarSeed !== persistedAvatarSeed
      || avatarBackgroundMode !== persistedAvatarBackgroundMode
      || (
        shouldPersistAvatarBackgroundColor
        && (
          !persistedShouldPersistAvatarBackgroundColor
          || avatarBackgroundColor !== persistedAvatarBackgroundColor
        )
      )
    ));

  useEffect(() => {
    return () => {
      if (pendingAvatar?.previewUrl) {
        URL.revokeObjectURL(pendingAvatar.previewUrl);
      }
    };
  }, [pendingAvatar]);

  useEffect(() => {
    setFullName(user?.full_name ?? '');
    setGeneratedAvatarEnabled(hasGeneratedTeamMemberAvatar(user?.avatar_style, user?.avatar_seed));
    setAvatarStyle(normalizeTeamMemberAvatarStyle(user?.avatar_style) ?? defaultAvatarStyle);
    setAvatarSeed(user?.avatar_seed ?? createTeamMemberAvatarSeed());
    setAvatarBackgroundMode(normalizeTeamMemberAvatarBackgroundMode(user?.avatar_background_mode));
    setAvatarBackgroundColor(normalizeTeamMemberAvatarBackgroundColor(user?.avatar_background_color) ?? TEAM_MEMBER_AVATAR_BACKGROUND_COLORS[0]);
  }, [defaultAvatarStyle, user?.avatar_background_color, user?.avatar_background_mode, user?.avatar_seed, user?.avatar_style, user?.full_name]);

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

    const buildQRCode = async () => {
      if (!setupProvisioning?.provisioning_uri) {
        setSetupQRCodeUrl('');
        return;
      }

      try {
        const dataUrl = await QRCode.toDataURL(setupProvisioning.provisioning_uri, {
          margin: 1,
          width: 256,
        });
        if (!cancelled) {
          setSetupQRCodeUrl(dataUrl);
        }
      } catch {
        if (!cancelled) {
          setSetupQRCodeUrl('');
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

  const copyRecoveryCodes = async (codes: string[]) => {
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      toast.success('Recovery codes copied');
    } catch {
      toast.error('Failed to copy recovery codes');
    }
  };

  const resetSetupDialog = () => {
    setSetupDialogOpen(false);
    setSetupPassword('');
    setSetupProvisioning(null);
    setSetupQRCodeUrl('');
    setSetupVerificationCode('');
  };

  const resetRecoveryDialog = () => {
    setRecoveryDialogOpen(false);
    setRecoveryPassword('');
    setRecoveryVerificationCode('');
    setRegeneratedRecoveryCodes(null);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await updateUser({
        full_name: fullName,
        avatar_style: generatedAvatarEnabled ? avatarStyle : '',
        avatar_seed: generatedAvatarEnabled ? avatarSeed : '',
        avatar_background_mode: generatedAvatarEnabled ? avatarBackgroundMode : '',
        avatar_background_color: shouldPersistAvatarBackgroundColor ? avatarBackgroundColor : '',
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.user.me });
      if (currentWorkspace?.id) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.members(currentWorkspace.id) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.assignableMembers(currentWorkspace.id) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(currentWorkspace.id) });
      }
      toast.success('Profile updated');
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to update profile';
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const resetPendingAvatar = () => {
    if (pendingAvatar?.previewUrl) {
      URL.revokeObjectURL(pendingAvatar.previewUrl);
    }
    setPendingAvatar(null);
    setAvatarDialogOpen(false);
  };

  const handleAvatarSave = async (file: File) => {
    setUploadingAvatar(true);
    const { data, error } = await authService.uploadAvatar(file);
    setUploadingAvatar(false);
    if (error || !data) {
      toast.error(error ?? 'Upload failed');
      return;
    }
    toast.success('Avatar updated');
    useAuthStore.setState({ user: data });
    if (currentWorkspace?.id) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.members(currentWorkspace.id) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(currentWorkspace.id) });
    }
    resetPendingAvatar();
  };

  const handleRemoveAvatar = async () => {
    setUploadingAvatar(true);
    const { data, error } = await authService.deleteAvatar();
    setUploadingAvatar(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Avatar removed');
      if (data) useAuthStore.setState({ user: data });
    }
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
  };

  const handleVerifyTwoFASetup = async (e: FormEvent) => {
    e.preventDefault();
    setSetupSubmitting(true);
    const { error } = await authService.verify2FASetup(setupVerificationCode);
    setSetupSubmitting(false);

    if (error) {
      toast.error(error);
      return;
    }

    syncTwoFAState(true);
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

  return (
    <div className="space-y-4">
      <AvatarPickerDialog
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        fullName={user?.full_name ?? undefined}
        email={user?.email ?? undefined}
        avatarUrl={user?.avatar_url}
        generatedAvatarEnabled={generatedAvatarEnabled}
        avatarStyle={avatarStyle}
        avatarSeed={avatarSeed}
        avatarBackgroundMode={avatarBackgroundMode}
        avatarBackgroundColor={avatarBackgroundColor}
        onGeneratedAvatarChange={async (state) => {
          setGeneratedAvatarEnabled(state.enabled);
          setAvatarStyle(state.style);
          setAvatarSeed(state.seed);
          setAvatarBackgroundMode(state.backgroundMode);
          setAvatarBackgroundColor(state.backgroundColor);
          setPickerOpen(false);
          // Auto-save avatar change immediately
          try {
            await updateUser({
              full_name: fullName,
              avatar_style: state.enabled ? state.style : '',
              avatar_seed: state.enabled ? state.seed : '',
              avatar_background_mode: state.enabled ? state.backgroundMode : '',
              avatar_background_color: state.enabled && state.backgroundMode === 'color' ? state.backgroundColor : '',
            });
            void queryClient.invalidateQueries({ queryKey: queryKeys.user.me });
            if (currentWorkspace?.id) {
              void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.members(currentWorkspace.id) });
              void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.assignableMembers(currentWorkspace.id) });
              void queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(currentWorkspace.id) });
            }
            toast.success(state.enabled ? 'Avatar updated' : 'Switched to initials');
          } catch {
            toast.error('Failed to update avatar');
          }
        }}
        onPhotoSelect={(file) => {
          if (pendingAvatar?.previewUrl) {
            URL.revokeObjectURL(pendingAvatar.previewUrl);
          }
          setPendingAvatar({ file, previewUrl: URL.createObjectURL(file) });
          setPickerOpen(false);
          setAvatarDialogOpen(true);
        }}
        onRemovePhoto={() => {
          handleRemoveAvatar();
          setPickerOpen(false);
        }}
        uploadingAvatar={uploadingAvatar}
      />
      <AvatarCropDialog
        open={avatarDialogOpen}
        imageUrl={pendingAvatar?.previewUrl ?? null}
        fileName={pendingAvatar?.file.name ?? 'avatar.png'}
        onOpenChange={(open) => {
          if (uploadingAvatar) return;
          if (!open) {
            resetPendingAvatar();
            return;
          }
          setAvatarDialogOpen(open);
        }}
        onSave={handleAvatarSave}
        saving={uploadingAvatar}
      />
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
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Enable Two-Factor Authentication</DialogTitle>
            <DialogDescription>
              Protect your Helpin account with a time-based code from Google Authenticator, Authy, 1Password, or another compatible app.
            </DialogDescription>
          </DialogHeader>

          {setupProvisioning ? (
            <div className="space-y-6">
              <div className="grid gap-6 md:grid-cols-[256px_1fr]">
                <div className="flex items-center justify-center rounded-xl border bg-white p-4">
                  {setupQRCodeUrl ? (
                    <img src={setupQRCodeUrl} alt="Authenticator QR code" className="h-64 w-64" />
                  ) : (
                    <div className="flex h-64 w-64 items-center justify-center text-sm text-muted-foreground">
                      Generating QR code...
                    </div>
                  )}
                </div>
                <div className="space-y-4">
                  <div className="rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground">
                    1. Scan the QR code with your authenticator app.
                    <br />
                    2. Save the recovery codes below somewhere secure.
                    <br />
                    3. Enter the 6-digit code from your app to activate 2FA.
                  </div>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <Label>Recovery Codes</Label>
                      <Button type="button" variant="outline" size="sm" onClick={() => void copyRecoveryCodes(setupProvisioning.recovery_codes)}>
                        <Copy01Icon className="mr-2 h-4 w-4" />
                        Copy all
                      </Button>
                    </div>
                    <RecoveryCodeList codes={setupProvisioning.recovery_codes} />
                  </div>
                </div>
              </div>

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
                  <Button type="button" variant="outline" onClick={resetSetupDialog}>
                    Cancel
                  </Button>
                  <Button type="submit" disabled={setupSubmitting}>
                    {setupSubmitting ? 'Verifying...' : 'Enable 2FA'}
                  </Button>
                </DialogFooter>
              </form>
            </div>
          ) : (
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
                  {setupSubmitting ? 'Preparing...' : 'Generate QR Code'}
                </Button>
              </DialogFooter>
            </form>
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

      <h2 className="text-xl font-semibold">Profile</h2>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <button
              type="button"
              className="relative group shrink-0 rounded-full"
              onClick={() => setPickerOpen(true)}
            >
              <Avatar className="h-16 w-16">
                {profileAvatarSrc && <AvatarImage src={profileAvatarSrc} alt={user?.full_name || 'Avatar'} />}
                <AvatarFallback className="text-lg">{initials}</AvatarFallback>
              </Avatar>
              <span className="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity">
                {uploadingAvatar ? (
                  <Loading01Icon className="h-5 w-5 text-white animate-spin" />
                ) : (
                  <Camera01Icon className="h-5 w-5 text-white" />
                )}
              </span>
            </button>
            <div>
              <CardTitle>{user?.full_name || 'User'}</CardTitle>
              <CardDescription className="flex items-center gap-1">
                <Mail01Icon className="h-3 w-3" />
                {user?.email}
              </CardDescription>
              <p className="mt-1 text-xs text-muted-foreground">
                Click your avatar to upload a photo or choose a generated style.
              </p>
            </div>
          </div>
        </CardHeader>
        <Separator />
        <CardContent className="pt-6">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="profile-name">Full Name</Label>
              <Input
                id="profile-name"
                value={fullName}
                onChange={e => setFullName(e.target.value)}
                placeholder="Your full name"
                required
              />
            </div>
            <div className="space-y-2">
              <Label>Email</Label>
              <Input value={user?.email ?? ''} disabled />
              <p className="text-xs text-muted-foreground">Email cannot be changed.</p>
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={saving || !hasProfileChanges}>
                {saving ? 'Saving...' : 'Save Changes'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Change Password</CardTitle>
          <CardDescription>Update your account password.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="current-password">Current Password</Label>
              <Input
                id="current-password"
                type="password"
                value={currentPassword}
                onChange={e => setCurrentPassword(e.target.value)}
                placeholder="Enter current password"
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">New Password</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                placeholder="Enter new password"
                required
                minLength={8}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password">Confirm New Password</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                placeholder="Confirm new password"
                required
                minLength={8}
              />
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={changingPassword}>
                {changingPassword ? 'Updating...' : 'Update Password'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <CardTitle className="flex items-center gap-2">
                <Shield02Icon className="h-5 w-5" />
                Two-Factor Authentication
              </CardTitle>
              <CardDescription>
                Add an authenticator app check to your Helpin sign-ins and keep recovery codes for backup access.
              </CardDescription>
            </div>
            <div className={`rounded-full px-3 py-1 text-xs font-medium ${twoFAEnabled ? 'bg-emerald-100 text-emerald-800' : 'bg-muted text-muted-foreground'}`}>
              {loadingTwoFAStatus ? 'Checking...' : twoFAEnabled ? 'Enabled' : 'Disabled'}
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {twoFAEnabled ? (
            <>
              <div className="rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground">
                <div className="flex items-center gap-2 font-medium text-foreground">
                  <SmartPhone01Icon className="h-4 w-4" />
                  Authenticator app required at login
                </div>
                <p className="mt-2">
                  Sign-ins now require your password plus a time-based one-time code. Recovery codes are single-use and should be stored offline.
                </p>
              </div>
              <div className="flex flex-wrap gap-3">
                <Button type="button" variant="outline" onClick={() => setRecoveryDialogOpen(true)}>
                  <ArrowReloadHorizontalIcon className="mr-2 h-4 w-4" />
                  Regenerate Recovery Codes
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

      {currentWorkspace && user && (
        <EmailAccountConnect workspaceId={currentWorkspace.id} memberId={user.id} />
      )}
    </div>
  );
}
