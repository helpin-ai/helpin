import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { MessageSquare, HelpCircle, CircleHelp } from 'lucide-react';
import { useChatSettings, useUpdateChatSettings } from '@/hooks/queries';
import { LINEAR_CARD_CLASS } from './settingsConstants';

const ICON_OPTIONS = [
  { value: 'chat_bubble', label: 'Chat Bubble', icon: MessageSquare },
  { value: 'question_mark', label: 'Question Mark', icon: HelpCircle },
  { value: 'help', label: 'Help', icon: CircleHelp },
];

export function ChatAppearanceTab({ workspaceId }: { workspaceId: string }) {
  const { data, isLoading } = useChatSettings(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);

  const [brandColor, setBrandColor] = useState('#6366F1');
  const [showBranding, setShowBranding] = useState(true);
  const [launcherPosition, setLauncherPosition] = useState('bottom_right');
  const [launcherIcon, setLauncherIcon] = useState('chat_bubble');
  const [welcomeMessage, setWelcomeMessage] = useState('');

  useEffect(() => {
    if (data?.settings) {
      const s = data.settings;
      setBrandColor(s.brand_color);
      setShowBranding(s.show_branding);
      setLauncherPosition(s.launcher_position);
      setLauncherIcon(s.launcher_icon);
      setWelcomeMessage(s.welcome_message);
    }
  }, [data]);

  const handleSave = () => {
    updateMutation.mutate({
      brand_color: brandColor,
      show_branding: showBranding,
      launcher_position: launcherPosition,
      launcher_icon: launcherIcon,
    }, {
      onSuccess: () => toast.success('Settings saved'),
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to save'),
    });
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  const LauncherIcon = ICON_OPTIONS.find(o => o.value === launcherIcon)?.icon ?? MessageSquare;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div />
        <Button onClick={handleSave} disabled={updateMutation.isPending} size="sm">
          {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      {/* Branding */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Branding</CardTitle>
          <CardDescription>Customize the widget's visual appearance.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-sm">Brand Color</Label>
            <div className="flex items-center gap-2">
              <div className="relative shrink-0">
                <div
                  className="h-9 w-9 rounded-md border shadow-sm cursor-pointer"
                  style={{ backgroundColor: brandColor }}
                />
                <input
                  type="color"
                  value={brandColor}
                  onChange={(e) => setBrandColor(e.target.value)}
                  className="absolute inset-0 cursor-pointer opacity-0"
                />
              </div>
              <Input
                value={brandColor}
                onChange={(e) => setBrandColor(e.target.value)}
                className="w-32 font-mono text-sm"
                placeholder="#6366F1"
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Show "Powered by" branding</Label>
              <p className="text-xs text-muted-foreground">Display branding in the widget footer.</p>
            </div>
            <Switch checked={showBranding} onCheckedChange={setShowBranding} />
          </div>
        </CardContent>
      </Card>

      {/* Launcher */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Launcher</CardTitle>
          <CardDescription>Configure the widget launcher button.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-sm">Position</Label>
            <Select value={launcherPosition} onValueChange={setLauncherPosition}>
              <SelectTrigger className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="bottom_right">Bottom Right</SelectItem>
                <SelectItem value="bottom_left">Bottom Left</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label className="text-sm">Icon</Label>
            <Select value={launcherIcon} onValueChange={setLauncherIcon}>
              <SelectTrigger className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ICON_OPTIONS.map(opt => (
                  <SelectItem key={opt.value} value={opt.value}>
                    <span className="flex items-center gap-2">
                      <opt.icon className="h-4 w-4" />
                      {opt.label}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Preview */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Preview</CardTitle>
          <CardDescription>A preview of how your widget will look.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative h-80 rounded-lg border bg-muted/20 overflow-hidden">
            {/* Mini widget preview */}
            <div className="absolute bottom-4 flex flex-col items-end gap-3" style={{ [launcherPosition === 'bottom_right' ? 'right' : 'left']: '16px' }}>
              {/* Chat window */}
              <div className="w-72 rounded-xl shadow-lg border bg-background overflow-hidden">
                <div className="px-4 py-3 text-white text-sm font-medium" style={{ backgroundColor: brandColor }}>
                  Chat with us
                </div>
                <div className="p-4 space-y-2">
                  <div className="bg-muted rounded-lg px-3 py-2 text-xs max-w-[200px]">
                    {welcomeMessage || 'Hi there! How can we help you today?'}
                  </div>
                </div>
                {showBranding && (
                  <div className="px-4 py-2 border-t text-center">
                    <span className="text-[10px] text-muted-foreground">Powered by Helpin</span>
                  </div>
                )}
              </div>

              {/* Launcher button */}
              <button
                className="h-12 w-12 rounded-full shadow-lg flex items-center justify-center text-white"
                style={{ backgroundColor: brandColor }}
              >
                <LauncherIcon className="h-5 w-5" />
              </button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
