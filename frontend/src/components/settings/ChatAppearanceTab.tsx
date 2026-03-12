import { useEffect, useState } from 'react';
import { Card, CardContent } from '@/components/ui/card';
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
import { WidgetPreview } from './WidgetPreview';

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

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={updateMutation.isPending} size="sm">
          {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      <Card className={LINEAR_CARD_CLASS}>
        <CardContent className="p-6">
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_380px]">
            {/* Left — Options */}
            <div className="space-y-6">
              {/* Brand Color */}
              <div className="space-y-2">
                <Label className="text-sm font-medium">Brand Color</Label>
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

              {/* Launcher Position */}
              <div className="space-y-2">
                <Label className="text-sm font-medium">Launcher Position</Label>
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

              {/* Launcher Icon */}
              <div className="space-y-2">
                <Label className="text-sm font-medium">Launcher Icon</Label>
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

              {/* Show Branding */}
              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm font-medium">Show "Powered by" branding</Label>
                  <p className="text-xs text-muted-foreground">Display branding in the widget footer.</p>
                </div>
                <Switch checked={showBranding} onCheckedChange={setShowBranding} />
              </div>
            </div>

            {/* Right — Live Preview */}
            <div className="hidden lg:block">
              <Label className="mb-2 block text-sm font-medium text-muted-foreground">Live Preview</Label>
              <WidgetPreview
                brandColor={brandColor}
                showBranding={showBranding}
                launcherPosition={launcherPosition}
                launcherIcon={launcherIcon}
                welcomeMessage={welcomeMessage}
              />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
