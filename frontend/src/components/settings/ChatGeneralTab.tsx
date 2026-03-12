import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { toast } from 'sonner';
import { Copy, Key, Code, MessageSquare, HelpCircle, CircleHelp } from 'lucide-react';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey } from '@/hooks/queries';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { WidgetPreview } from './WidgetPreview';
import { API_BASE } from '@/lib/api';

const ICON_OPTIONS = [
  { value: 'chat_bubble', label: 'Chat Bubble', icon: MessageSquare },
  { value: 'question_mark', label: 'Question Mark', icon: HelpCircle },
  { value: 'help', label: 'Help', icon: CircleHelp },
];

export function ChatGeneralTab({ workspaceId }: { workspaceId: string }) {
  const { data, isLoading } = useChatSettings(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateMutation = useRegenerateWidgetKey(workspaceId);

  // Identity & CRM state
  const [requireEmail, setRequireEmail] = useState(true);
  const [requireName, setRequireName] = useState(false);
  const [welcomeMessage, setWelcomeMessage] = useState('');
  const [autoCreateContact, setAutoCreateContact] = useState(true);
  const [lifecycleStage, setLifecycleStage] = useState('subscriber');
  const [autoPromote, setAutoPromote] = useState(false);

  // Appearance state
  const [brandColor, setBrandColor] = useState('#6366F1');
  const [showBranding, setShowBranding] = useState(true);
  const [launcherPosition, setLauncherPosition] = useState('bottom_right');
  const [launcherIcon, setLauncherIcon] = useState('chat_bubble');

  useEffect(() => {
    if (data?.settings) {
      const s = data.settings;
      setRequireEmail(s.require_email_before_chat);
      setRequireName(s.require_name_after_email);
      setWelcomeMessage(s.welcome_message);
      setAutoCreateContact(s.auto_create_crm_contact);
      setLifecycleStage(s.default_lifecycle_stage);
      setAutoPromote(s.auto_promote_to_lead);
      setBrandColor(s.brand_color);
      setShowBranding(s.show_branding);
      setLauncherPosition(s.launcher_position);
      setLauncherIcon(s.launcher_icon);
    }
  }, [data]);

  const handleSave = () => {
    updateMutation.mutate({
      require_email_before_chat: requireEmail,
      require_name_after_email: requireName,
      welcome_message: welcomeMessage,
      auto_create_crm_contact: autoCreateContact,
      default_lifecycle_stage: lifecycleStage,
      auto_promote_to_lead: autoPromote,
      brand_color: brandColor,
      show_branding: showBranding,
      launcher_position: launcherPosition,
      launcher_icon: launcherIcon,
    }, {
      onSuccess: () => toast.success('Settings saved'),
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to save'),
    });
  };

  const handleRegenerate = () => {
    regenerateMutation.mutate(undefined, {
      onSuccess: () => toast.success('Widget key regenerated'),
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to regenerate'),
    });
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    toast.success(`${label} copied`);
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  const widgetKey = data?.widget_key ?? '';
  const embedSnippet = `<script src="${API_BASE.replace('/api', '')}/widget.js" data-widget-key="${widgetKey}"></script>`;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div />
        <Button onClick={handleSave} disabled={updateMutation.isPending} size="sm">
          {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      {/* Widget Installation */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <Code className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Widget Installation</CardTitle>
          </div>
          <CardDescription>Embed the chat widget on your website.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-sm">Widget Key</Label>
            <div className="flex items-center gap-2">
              <Input value={widgetKey} readOnly className="font-mono text-sm" />
              <Button type="button" variant="outline" size="icon" className="shrink-0" onClick={() => copyToClipboard(widgetKey, 'Widget key')}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>

          <div className="space-y-2">
            <Label className="text-sm">Embed Snippet</Label>
            <div className="relative">
              <pre className="rounded-md border bg-muted/50 p-3 text-xs font-mono overflow-x-auto">{embedSnippet}</pre>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute top-2 right-2 h-7 w-7"
                onClick={() => copyToClipboard(embedSnippet, 'Snippet')}
              >
                <Copy className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" className="gap-1.5">
                <Key className="h-3.5 w-3.5" />
                Regenerate Key
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Regenerate Widget Key?</AlertDialogTitle>
                <AlertDialogDescription>
                  This will invalidate the current widget key. You will need to update the embed snippet on all pages where it is installed.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={handleRegenerate} disabled={regenerateMutation.isPending}>
                  {regenerateMutation.isPending ? 'Regenerating...' : 'Regenerate'}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardContent>
      </Card>

      {/* Identity Capture */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Identity Capture</CardTitle>
          <CardDescription>Control what information is collected before starting a chat.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Require email before chat</Label>
              <p className="text-xs text-muted-foreground">Visitors must enter their email to start a conversation.</p>
            </div>
            <Switch checked={requireEmail} onCheckedChange={setRequireEmail} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Require name after email</Label>
              <p className="text-xs text-muted-foreground">Also ask for the visitor's name.</p>
            </div>
            <Switch checked={requireName} onCheckedChange={setRequireName} disabled={!requireEmail} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="welcome-msg" className="text-sm">Welcome Message</Label>
            <Input
              id="welcome-msg"
              value={welcomeMessage}
              onChange={(e) => setWelcomeMessage(e.target.value)}
              placeholder="Hi there! How can we help you today?"
            />
          </div>
        </CardContent>
      </Card>

      {/* Appearance + Live Preview */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Appearance</CardTitle>
          <CardDescription>Customize the widget's visual appearance.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_380px]">
            <div className="space-y-6">
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

              <div className="flex items-center justify-between">
                <div>
                  <Label className="text-sm font-medium">Show "Powered by" branding</Label>
                  <p className="text-xs text-muted-foreground">Display branding in the widget footer.</p>
                </div>
                <Switch checked={showBranding} onCheckedChange={setShowBranding} />
              </div>
            </div>

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

      {/* CRM Integration */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">CRM Integration</CardTitle>
          <CardDescription>Automatically create and manage CRM contacts from chat conversations.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Auto-create CRM contact</Label>
              <p className="text-xs text-muted-foreground">Create a contact when a visitor provides their email.</p>
            </div>
            <Switch checked={autoCreateContact} onCheckedChange={setAutoCreateContact} />
          </div>

          <div className="space-y-2">
            <Label className="text-sm">Default Lifecycle Stage</Label>
            <Select value={lifecycleStage} onValueChange={setLifecycleStage}>
              <SelectTrigger className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="subscriber">Subscriber</SelectItem>
                <SelectItem value="lead">Lead</SelectItem>
                <SelectItem value="opportunity">Opportunity</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm">Auto-promote to lead</Label>
              <p className="text-xs text-muted-foreground">Promote subscribers to leads after their first conversation.</p>
            </div>
            <Switch checked={autoPromote} onCheckedChange={setAutoPromote} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
