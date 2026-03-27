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
import { getInitials } from '@/lib/utils';
import { Camera, Loader2, Mail, Trash2 } from 'lucide-react';
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
  const [fullName, setFullName] = useState(user?.full_name ?? '');
  const [saving, setSaving] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [pendingAvatar, setPendingAvatar] = useState<PendingAvatarFile | null>(null);
  const [avatarDialogOpen, setAvatarDialogOpen] = useState(false);

  const initials = getInitials(user?.full_name || user?.email);

  useEffect(() => {
    return () => {
      if (pendingAvatar?.previewUrl) {
        URL.revokeObjectURL(pendingAvatar.previewUrl);
      }
    };
  }, [pendingAvatar]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await updateUser({ full_name: fullName });
      toast.success('Profile updated');
    } catch {
      toast.error('Failed to update profile');
    }
    setSaving(false);
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
                {user?.avatar_url && <AvatarImage src={user.avatar_url} alt={user.full_name || 'Avatar'} />}
                <AvatarFallback className="text-lg">{initials}</AvatarFallback>
              </Avatar>
              <label className="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
                {uploadingAvatar ? (
                  <Loader2 className="h-5 w-5 text-white animate-spin" />
                ) : (
                  <Camera className="h-5 w-5 text-white" />
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
                <Mail className="h-3 w-3" />
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
                  <Trash2 className="h-3 w-3 mr-1" />
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
            <div className="flex justify-end">
              <Button type="submit" disabled={saving}>
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
