import { useEffect, useState, type FormEvent } from 'react';
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
import { Camera01Icon, Loading01Icon, Mail01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { AvatarCropDialog } from '@/components/profile/AvatarCropDialog';
import { AvatarPickerDialog } from '@/components/profile/AvatarPickerDialog';
import { queryClient } from '@/lib/queryClient';
import { queryKeys } from '@/lib/queryKeys';

type PendingAvatarFile = {
  file: File;
  previewUrl: string;
};

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

      {currentWorkspace && user && (
        <EmailAccountConnect workspaceId={currentWorkspace.id} memberId={user.id} />
      )}
    </div>
  );
}
