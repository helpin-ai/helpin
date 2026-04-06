import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { authService } from '@/lib/services/authService';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
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
import { cn, getInitials } from '@/lib/utils';
import { Camera01Icon, Loading01Icon, Mail01Icon, Delete01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { AvatarCropDialog } from '@/components/profile/AvatarCropDialog';
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
  const avatarInputRef = useRef<HTMLInputElement>(null);
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
    if (avatarInputRef.current) {
      avatarInputRef.current.value = '';
    }
  };

  const handleAvatarFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file');
      e.target.value = '';
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2MB');
      e.target.value = '';
      return;
    }

    if (pendingAvatar?.previewUrl) {
      URL.revokeObjectURL(pendingAvatar.previewUrl);
    }

    setPendingAvatar({
      file,
      previewUrl: URL.createObjectURL(file),
    });
    setAvatarDialogOpen(true);
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

  const handleEnableGeneratedAvatar = () => {
    setGeneratedAvatarEnabled(true);
    setAvatarStyle((current) => normalizeTeamMemberAvatarStyle(current) ?? defaultAvatarStyle);
    setAvatarSeed((current) => current || createTeamMemberAvatarSeed());
    setAvatarBackgroundMode((current) => normalizeTeamMemberAvatarBackgroundMode(current));
    setAvatarBackgroundColor((current) => normalizeTeamMemberAvatarBackgroundColor(current) ?? TEAM_MEMBER_AVATAR_BACKGROUND_COLORS[0]);
  };

  const handleShuffleGeneratedAvatar = () => {
    setGeneratedAvatarEnabled(true);
    setAvatarSeed(createTeamMemberAvatarSeed());
  };

  const handleAvatarBackgroundModeChange = (value: TeamMemberAvatarBackgroundMode) => {
    setAvatarBackgroundMode(value);
    if (value === 'auto') {
      setAvatarBackgroundColor(TEAM_MEMBER_AVATAR_BACKGROUND_COLORS[0]);
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
            <div className="relative group">
              <Avatar className="h-16 w-16">
                {profileAvatarSrc && <AvatarImage src={profileAvatarSrc} alt={user?.full_name || 'Avatar'} />}
                <AvatarFallback className="text-lg">{initials}</AvatarFallback>
              </Avatar>
              <label className="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
                {uploadingAvatar ? (
                  <Loading01Icon className="h-5 w-5 text-white animate-spin" />
                ) : (
                  <Camera01Icon className="h-5 w-5 text-white" />
                )}
                <input
                  ref={avatarInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={handleAvatarFileSelect}
                  disabled={uploadingAvatar}
                />
              </label>
            </div>
            <div>
              <CardTitle>{user?.full_name || 'User'}</CardTitle>
              <CardDescription className="flex items-center gap-1">
                <Mail01Icon className="h-3 w-3" />
                {user?.email}
              </CardDescription>
              <p className="mt-1 text-xs text-muted-foreground">
                Upload a square-friendly image, then drag and zoom before saving.
              </p>
              {user?.avatar_url && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 mt-1 text-xs text-destructive hover:text-destructive p-0"
                  onClick={handleRemoveAvatar}
                  disabled={uploadingAvatar}
                >
                  <Delete01Icon className="h-3 w-3 mr-1" />
                  Remove photo
                </Button>
              )}
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
            <div className="space-y-3 rounded-xl border bg-muted/20 p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <Label>Generated avatar</Label>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Fallback order is uploaded photo, then generated avatar, then initials.
                  </p>
                </div>
                {generatedAvatarEnabled ? (
                  <Button type="button" variant="ghost" size="sm" onClick={() => setGeneratedAvatarEnabled(false)}>
                    Use initials instead
                  </Button>
                ) : (
                  <Button type="button" variant="outline" size="sm" onClick={handleEnableGeneratedAvatar}>
                    Use generated avatar
                  </Button>
                )}
              </div>

              {generatedAvatarEnabled && (
                <div className="flex flex-col gap-4 md:flex-row md:items-center">
                  <div className="flex items-center gap-3">
                    <Avatar className="h-14 w-14">
                      {profileAvatarSrc && <AvatarImage src={profileAvatarSrc} alt="Generated avatar preview" />}
                      <AvatarFallback>{initials}</AvatarFallback>
                    </Avatar>
                    <Button type="button" variant="outline" size="sm" onClick={handleShuffleGeneratedAvatar}>
                      Shuffle
                    </Button>
                  </div>
                  <div className="min-w-0 flex-1 space-y-2">
                    <Label htmlFor="avatar-style">Style</Label>
                    <Select value={avatarStyle} onValueChange={(value) => setAvatarStyle(value as typeof defaultAvatarStyle)}>
                      <SelectTrigger id="avatar-style">
                        <SelectValue placeholder="Choose a style" />
                      </SelectTrigger>
                      <SelectContent>
                        {TEAM_MEMBER_AVATAR_STYLES.map((style) => (
                          <SelectItem key={style.value} value={style.value}>
                            {style.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              )}
              {generatedAvatarEnabled && avatarStyle === 'personas' && (
                <div className="flex flex-col gap-3 md:flex-row md:items-end">
                  <div className="space-y-2 md:w-52">
                    <Label htmlFor="avatar-background-mode">Background</Label>
                    <Select value={avatarBackgroundMode} onValueChange={(value) => handleAvatarBackgroundModeChange(value as TeamMemberAvatarBackgroundMode)}>
                      <SelectTrigger id="avatar-background-mode">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="auto">Auto</SelectItem>
                        <SelectItem value="color">Color</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  {avatarBackgroundMode === 'color' && (
                    <div className="space-y-2 md:flex-1">
                      <Label>Color</Label>
                      <div className="flex flex-wrap items-center pl-3">
                        {TEAM_MEMBER_AVATAR_BACKGROUND_COLORS.map((color, index) => (
                          <button
                            key={color}
                            type="button"
                            aria-label={`Select avatar background ${color}`}
                            className={cn(
                              'relative h-8 w-8 rounded-full border-2 border-background ring-2 ring-offset-2 ring-offset-background transition-transform',
                              index === 0 ? 'ml-0' : '-ml-3',
                              avatarBackgroundColor === color
                                ? 'z-20 scale-110 ring-foreground'
                                : 'z-0 ring-transparent hover:z-10 hover:scale-105'
                            )}
                            style={{ backgroundColor: color }}
                            onClick={() => setAvatarBackgroundColor(color)}
                          />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
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
