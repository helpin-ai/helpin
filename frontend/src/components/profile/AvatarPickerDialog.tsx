import { useRef, useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Camera01Icon, Delete01Icon, Image01Icon, Loading01Icon, Upload01Icon } from '@/lib/icons';
import {
  TEAM_MEMBER_AVATAR_STYLES,
  TEAM_MEMBER_AVATAR_BACKGROUND_COLORS,
  type TeamMemberAvatarBackgroundMode,
  type TeamMemberAvatarStyle,
  createTeamMemberAvatarSeed,
  resolveTeamMemberAvatarSrc,
} from '@/lib/teamMemberAvatar';
import { cn, getInitials } from '@/lib/utils';
import { toast } from 'sonner';

type AvatarPickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Current user state
  fullName?: string;
  email?: string;
  avatarUrl?: string | null;
  // Generated avatar state
  generatedAvatarEnabled: boolean;
  avatarStyle: TeamMemberAvatarStyle;
  avatarSeed: string;
  avatarBackgroundMode: TeamMemberAvatarBackgroundMode;
  avatarBackgroundColor: string;
  // Callbacks
  onGeneratedAvatarChange: (state: {
    enabled: boolean;
    style: TeamMemberAvatarStyle;
    seed: string;
    backgroundMode: TeamMemberAvatarBackgroundMode;
    backgroundColor: string;
  }) => void;
  onPhotoSelect: (file: File) => void;
  onRemovePhoto: () => void;
  uploadingAvatar: boolean;
};

export function AvatarPickerDialog({
  open,
  onOpenChange,
  fullName,
  email,
  avatarUrl,
  generatedAvatarEnabled,
  avatarStyle,
  avatarSeed,
  avatarBackgroundMode,
  avatarBackgroundColor,
  onGeneratedAvatarChange,
  onPhotoSelect,
  onRemovePhoto,
  uploadingAvatar,
}: AvatarPickerDialogProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [localStyle, setLocalStyle] = useState(avatarStyle);
  const [localSeed, setLocalSeed] = useState(avatarSeed);
  const [localBgMode, setLocalBgMode] = useState(avatarBackgroundMode);
  const [localBgColor, setLocalBgColor] = useState(avatarBackgroundColor);

  const initials = getInitials(fullName || email);

  // Sync local state when dialog opens
  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      setLocalStyle(avatarStyle);
      setLocalSeed(avatarSeed);
      setLocalBgMode(avatarBackgroundMode);
      setLocalBgColor(avatarBackgroundColor);
    }
    onOpenChange(nextOpen);
  };

  const generatedAvatarSrc = resolveTeamMemberAvatarSrc({
    avatarStyle: localStyle,
    avatarSeed: localSeed,
    avatarBackgroundMode: 'color',
    avatarBackgroundColor: localBgColor,
    fallbackSeed: fullName ?? email,
  });

  const currentAvatarSrc = resolveTeamMemberAvatarSrc({
    avatarUrl,
    avatarStyle: generatedAvatarEnabled ? avatarStyle : undefined,
    avatarSeed: generatedAvatarEnabled ? avatarSeed : undefined,
    avatarBackgroundMode: generatedAvatarEnabled ? avatarBackgroundMode : undefined,
    avatarBackgroundColor: generatedAvatarEnabled ? avatarBackgroundColor : undefined,
    fallbackSeed: fullName ?? email,
  });

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
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
    onPhotoSelect(file);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleShuffle = () => {
    setLocalSeed(createTeamMemberAvatarSeed());
  };


  const handleUseGenerated = () => {
    onGeneratedAvatarChange({
      enabled: true,
      style: localStyle,
      seed: localSeed,
      backgroundMode: 'color',
      backgroundColor: localBgColor,
    });
  };

  const handleUseInitials = () => {
    onGeneratedAvatarChange({
      enabled: false,
      style: localStyle,
      seed: localSeed,
      backgroundMode: localBgMode,
      backgroundColor: localBgColor,
    });
  };

  const defaultTab = avatarUrl ? 'upload' : 'generated';

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Change avatar</DialogTitle>
          <DialogDescription>
            Upload a photo, choose a generated avatar, or use your initials.
          </DialogDescription>
        </DialogHeader>

        {/* Current avatar preview */}
        <div className="flex items-center gap-4">
          <Avatar className="h-16 w-16 border border-border/80">
            {currentAvatarSrc && <AvatarImage src={currentAvatarSrc} alt={fullName || 'Avatar'} />}
            <AvatarFallback className="text-lg">{initials}</AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="text-sm font-medium truncate">{fullName || 'User'}</p>
            <p className="text-xs text-muted-foreground">
              {avatarUrl ? 'Using uploaded photo' : generatedAvatarEnabled ? 'Using generated avatar' : 'Using initials'}
            </p>
          </div>
        </div>

        <Tabs defaultValue={defaultTab} className="w-full">
          <TabsList className="w-full">
            <TabsTrigger value="upload" className="flex-1 gap-1.5">
              <Camera01Icon className="h-3.5 w-3.5" />
              Upload photo
            </TabsTrigger>
            <TabsTrigger value="generated" className="flex-1 gap-1.5">
              <Image01Icon className="h-3.5 w-3.5" />
              Avatar
            </TabsTrigger>
          </TabsList>

          {/* Upload photo tab */}
          <TabsContent value="upload" className="space-y-4 pt-2">
            <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed border-border/60 bg-muted/20 p-6">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <Upload01Icon className="h-5 w-5 text-muted-foreground" />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium">Upload a photo</p>
                <p className="text-xs text-muted-foreground">
                  Square-friendly image, under 2MB. You can crop after selecting.
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadingAvatar}
              >
                {uploadingAvatar ? (
                  <>
                    <Loading01Icon className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                    Uploading...
                  </>
                ) : (
                  'Choose file'
                )}
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleFileChange}
                disabled={uploadingAvatar}
              />
            </div>
            {avatarUrl && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="w-full text-destructive hover:text-destructive"
                onClick={onRemovePhoto}
                disabled={uploadingAvatar}
              >
                <Delete01Icon className="h-3.5 w-3.5 mr-1.5" />
                Remove current photo
              </Button>
            )}
          </TabsContent>

          {/* Generated avatar tab */}
          <TabsContent value="generated" className="space-y-4 pt-2">
            <div className="flex items-center gap-4">
              <Avatar className="h-16 w-16 border border-border/80">
                {generatedAvatarSrc && <AvatarImage src={generatedAvatarSrc} alt="Generated avatar preview" />}
                <AvatarFallback>{initials}</AvatarFallback>
              </Avatar>
              <div className="flex flex-col gap-1.5">
                <Button type="button" variant="outline" size="sm" onClick={handleShuffle}>
                  New face
                </Button>
                <p className="text-[11px] text-muted-foreground">Generate a different look</p>
              </div>
            </div>

            <div className="flex items-end gap-3">
              <div className="flex-1 space-y-2">
                <Label htmlFor="picker-avatar-style">Style</Label>
                <Select value={localStyle} onValueChange={(value) => setLocalStyle(value as TeamMemberAvatarStyle)}>
                  <SelectTrigger id="picker-avatar-style">
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

            <div className="space-y-2">
              <Label>Background color</Label>
              <div className="flex flex-wrap gap-1.5">
                {TEAM_MEMBER_AVATAR_BACKGROUND_COLORS.map((color) => (
                  <button
                    key={color}
                    type="button"
                    aria-label={`Select background ${color}`}
                    className={cn(
                      'h-7 w-7 rounded-full border-2 border-background ring-2 ring-offset-1 ring-offset-background transition-transform',
                      localBgColor === color
                        ? 'scale-110 ring-foreground'
                        : 'ring-transparent hover:scale-105',
                    )}
                    style={{ backgroundColor: color }}
                    onClick={() => setLocalBgColor(color)}
                  />
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2 pt-1">
              <Button type="button" size="sm" onClick={handleUseGenerated}>
                Use this avatar
              </Button>
              {generatedAvatarEnabled && (
                <Button type="button" variant="ghost" size="sm" onClick={handleUseInitials}>
                  Use initials instead
                </Button>
              )}
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
