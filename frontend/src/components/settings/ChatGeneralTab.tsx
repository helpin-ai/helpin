import { useEffect, useState, useRef, type ReactNode } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';
import { toast } from 'sonner';
import { Copy, Key, Code, MessageSquare, HelpCircle, CircleHelp, ImageIcon, Monitor, Sun, Moon, ArrowLeft } from 'lucide-react';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey } from '@/hooks/queries';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { WidgetPreview } from './WidgetPreview';
import { API_BASE } from '@/lib/api';

const ICON_OPTIONS = [
  { value: 'chat_bubble', label: 'Chat Bubble', icon: MessageSquare },
  { value: 'question_mark', label: 'Question Mark', icon: HelpCircle },
  { value: 'help', label: 'Help', icon: CircleHelp },
];

const COLOR_SCHEME_OPTIONS = [
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
];

type SettingsView = 'main' | 'branding';

/* ── Two-column layout shell ─────────────────────────────────────────── */

function PreviewLayout({ children, preview }: { children: ReactNode; preview: ReactNode }) {
  return (
    <div className="grid items-start gap-8 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div className="min-w-0">
        {children}
      </div>

      <aside className="hidden self-start xl:block xl:sticky xl:top-0">
        <div className="h-[calc(100svh-8rem)] min-h-[36rem] overflow-y-auto overscroll-contain">
          <div className="h-full pr-1">
            {preview}
          </div>
        </div>
      </aside>
    </div>
  );
}

/* ── Main component ──────────────────────────────────────────────────── */

export function ChatGeneralTab({ workspaceId }: { workspaceId: string }) {
  const { data, isLoading } = useChatSettings(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateMutation = useRegenerateWidgetKey(workspaceId);

  const [view, setView] = useState<SettingsView>('main');

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
  const [colorScheme, setColorScheme] = useState('light');
  const [buttonColor, setButtonColor] = useState('#000000');
  const [buttonIconColor, setButtonIconColor] = useState('#FFFFFF');
  const [logoUrl, setLogoUrl] = useState('');

  const logoInputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);

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
      setColorScheme(s.color_scheme || 'light');
      setButtonColor(s.button_color || '#000000');
      setButtonIconColor(s.button_icon_color || '#FFFFFF');
      setLogoUrl(s.logo_url || '');
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
      color_scheme: colorScheme,
      button_color: buttonColor,
      button_icon_color: buttonIconColor,
      logo_url: logoUrl,
    }, {
      onSuccess: () => toast.success('Settings saved'),
      onError: (err: unknown) => toast.error(err instanceof Error ? err.message : 'Failed to save'),
    });
  };

  const handleRegenerate = () => {
    regenerateMutation.mutate(undefined, {
      onSuccess: () => toast.success('Widget key regenerated'),
      onError: (err: unknown) => toast.error(err instanceof Error ? err.message : 'Failed to regenerate'),
    });
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    toast.success(`${label} copied`);
  };

  const handleLogoFileSelect = (file: File) => {
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file (PNG, JPG, SVG)');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2MB');
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setLogoUrl(reader.result as string);
    };
    reader.readAsDataURL(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = () => {
    setDragOver(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) handleLogoFileSelect(file);
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

  // Shared preview element used by both views
  const previewElement = (
    <WidgetPreview
      brandColor={brandColor}
      showBranding={showBranding}
      launcherPosition={launcherPosition}
      launcherIcon={launcherIcon}
      welcomeMessage={welcomeMessage}
      colorScheme={colorScheme}
      buttonColor={buttonColor}
      buttonIconColor={buttonIconColor}
      logoUrl={logoUrl}
    />
  );

  /* ── Branding sub-page ─────────────────────────────────────────────── */

  if (view === 'branding') {
    return (
      <PreviewLayout preview={previewElement}>
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <button
              onClick={() => setView('main')}
              className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              Branding
            </button>
            <Button onClick={handleSave} disabled={updateMutation.isPending} size="sm">
              {updateMutation.isPending ? 'Saving...' : 'Save changes'}
            </Button>
          </div>

          {/* Logo */}
          <div className="space-y-3">
            <div>
              <Label className="text-sm font-medium">Logo</Label>
              <p className="text-xs text-muted-foreground mt-0.5">
                The image that appears at the top of your widget. Recommended size: 512x512 pixels.
              </p>
            </div>
            {logoUrl ? (
              <div className="flex items-center gap-4">
                <img
                  src={logoUrl}
                  alt="Widget logo"
                  className="h-16 w-16 rounded-lg border object-cover"
                />
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => logoInputRef.current?.click()}>
                    Change
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setLogoUrl('')}>
                    Remove
                  </Button>
                </div>
              </div>
            ) : (
              <div
                className={`flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 transition-colors cursor-pointer ${
                  dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-muted-foreground/50'
                }`}
                onClick={() => logoInputRef.current?.click()}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
              >
                <ImageIcon className="h-8 w-8 text-muted-foreground/50" />
                <div className="text-center">
                  <p className="text-sm text-muted-foreground">
                    Drop an image or{' '}
                    <span className="text-primary font-medium">click to browse</span>
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">Supports PNG, JPG, SVG</p>
                </div>
              </div>
            )}
            <input
              ref={logoInputRef}
              type="file"
              accept="image/png,image/jpeg,image/svg+xml"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) handleLogoFileSelect(file);
                e.target.value = '';
              }}
            />
          </div>

          {/* Primary Color */}
          <div className="space-y-2">
            <div>
              <Label className="text-sm font-medium">Primary color</Label>
              <p className="text-xs text-muted-foreground mt-0.5">
                The main color used for links and accents in your widget.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative shrink-0">
                <div className="h-9 w-9 rounded-md border shadow-sm cursor-pointer" style={{ backgroundColor: brandColor }} />
                <input type="color" value={brandColor} onChange={(e) => setBrandColor(e.target.value)} className="absolute inset-0 cursor-pointer opacity-0" />
              </div>
              <Input value={brandColor} onChange={(e) => setBrandColor(e.target.value)} className="w-32 font-mono text-sm" placeholder="#6366F1" />
            </div>
          </div>

          {/* Color Scheme */}
          <div className="space-y-2">
            <div>
              <Label className="text-sm font-medium">Color Scheme</Label>
              <p className="text-xs text-muted-foreground mt-0.5">Choose the default color scheme for your widget.</p>
            </div>
            <div className="flex items-center gap-1 p-1 rounded-lg border bg-muted/30 w-fit">
              {COLOR_SCHEME_OPTIONS.map(opt => (
                <button
                  key={opt.value}
                  onClick={() => setColorScheme(opt.value)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm transition-colors ${
                    colorScheme === opt.value
                      ? 'bg-background shadow-sm font-medium text-foreground'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <opt.icon className="h-3.5 w-3.5" />
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* Button Color */}
          <div className="space-y-2">
            <div>
              <Label className="text-sm font-medium">Button color</Label>
              <p className="text-xs text-muted-foreground mt-0.5">Background color of the floating widget button.</p>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative shrink-0">
                <div className="h-9 w-9 rounded-md border shadow-sm cursor-pointer" style={{ backgroundColor: buttonColor }} />
                <input type="color" value={buttonColor} onChange={(e) => setButtonColor(e.target.value)} className="absolute inset-0 cursor-pointer opacity-0" />
              </div>
              <Input value={buttonColor} onChange={(e) => setButtonColor(e.target.value)} className="w-32 font-mono text-sm" placeholder="#000000" />
            </div>
          </div>

          {/* Button Icon Color */}
          <div className="space-y-2">
            <div>
              <Label className="text-sm font-medium">Button icon color</Label>
              <p className="text-xs text-muted-foreground mt-0.5">Icon color of the floating widget button.</p>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative shrink-0">
                <div className="h-9 w-9 rounded-md border shadow-sm cursor-pointer" style={{ backgroundColor: buttonIconColor }} />
                <input type="color" value={buttonIconColor} onChange={(e) => setButtonIconColor(e.target.value)} className="absolute inset-0 cursor-pointer opacity-0" />
              </div>
              <Input value={buttonIconColor} onChange={(e) => setButtonIconColor(e.target.value)} className="w-32 font-mono text-sm" placeholder="#FFFFFF" />
            </div>
          </div>

          {/* Show Powered By */}
          <div className="flex items-center justify-between pt-2">
            <div>
              <Label className="text-sm font-medium">Show "Powered by Helpin"</Label>
              <p className="text-xs text-muted-foreground mt-0.5">Display branding in the widget footer.</p>
            </div>
            <Switch checked={showBranding} onCheckedChange={setShowBranding} />
          </div>
        </div>
      </PreviewLayout>
    );
  }

  /* ── Main settings view ────────────────────────────────────────────── */

  return (
    <PreviewLayout preview={previewElement}>
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

        {/* Appearance */}
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <CardTitle className="text-base">Appearance</CardTitle>
            <CardDescription>Customize the widget's visual appearance.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Branding sub-page link */}
            <button
              onClick={() => setView('branding')}
              className="w-full flex items-center justify-between rounded-lg border p-4 hover:bg-muted/50 transition-colors text-left"
            >
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md border bg-muted/50">
                  <ImageIcon className="h-4 w-4 text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">Branding</p>
                  <p className="text-xs text-muted-foreground">Logo, colors, color scheme</p>
                </div>
              </div>
              <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-muted-foreground"><path d="m9 18 6-6-6-6"/></svg>
            </button>

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
    </PreviewLayout>
  );
}
