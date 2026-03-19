import { useEffect, useState, useRef, type ReactNode } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { Copy, Code, MessageSquare, HelpCircle, CircleHelp, ImageIcon, Monitor, Sun, Moon, KeyRound } from 'lucide-react';
import { useChatSettings, useUpdateChatSettings, useRegenerateWidgetKey, useDocsSpaces } from '@/hooks/queries';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { WidgetPreview } from './WidgetPreview';
import { CodeBlock } from '@/components/ui/code-block';
import { BrandColorPicker } from '@/components/pm/ColorPicker';
import { useWorkspaceStore } from '@/stores/workspaceStore';

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
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data, isLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const updateMutation = useUpdateChatSettings(workspaceId);
  const regenerateKeyMutation = useRegenerateWidgetKey(workspaceId);

  const [snippetTab, setSnippetTab] = useState<'basic' | 'advanced'>('basic');

  // Identity state
  const [requireEmail, setRequireEmail] = useState(true);
  const [requirePhone, setRequirePhone] = useState(false);
  const [welcomeMessage, setWelcomeMessage] = useState('');

  // Appearance state
  const [brandColor, setBrandColor] = useState('#6366F1');
  const [showBranding, setShowBranding] = useState(true);
  const [launcherPosition, setLauncherPosition] = useState('bottom_right');
  const [launcherIcon, setLauncherIcon] = useState('chat_bubble');
  const [colorScheme, setColorScheme] = useState('light');
  const [buttonColor, setButtonColor] = useState('#000000');
  const [buttonIconColor, setButtonIconColor] = useState('#FFFFFF');
  const [logoUrl, setLogoUrl] = useState('');
  const [widgetName, setWidgetName] = useState('');
  const [widgetAvatarUrl, setWidgetAvatarUrl] = useState('');
  const [widgetHelpSpaceIds, setWidgetHelpSpaceIds] = useState<string[]>([]);

  const logoInputRef = useRef<HTMLInputElement>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);

  useEffect(() => {
    if (data?.settings) {
      const s = data.settings;
      setRequireEmail(s.require_email_before_chat);
      setRequirePhone(s.require_phone_after_email);
      setWelcomeMessage(s.welcome_message);
      setBrandColor(s.brand_color);
      setShowBranding(s.show_branding);
      setLauncherPosition(s.launcher_position);
      setLauncherIcon(s.launcher_icon);
      setColorScheme(s.color_scheme || 'light');
      setButtonColor(s.button_color || '#000000');
      setButtonIconColor(s.button_icon_color || '#FFFFFF');
      setLogoUrl(s.logo_url || '');
      setWidgetName(s.widget_name || '');
      setWidgetAvatarUrl(s.widget_avatar_url || '');
      setWidgetHelpSpaceIds(s.widget_help_space_ids || []);
    }
  }, [data]);

  const handleSave = () => {
    updateMutation.mutate({
      require_email_before_chat: requireEmail,
      require_phone_after_email: requirePhone,
      welcome_message: welcomeMessage,
      widget_name: widgetName,
      widget_avatar_url: widgetAvatarUrl,
      widget_help_space_ids: widgetHelpSpaceIds,
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

  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');

  const toggleHelpSpace = (spaceId: string, enabled: boolean) => {
    setWidgetHelpSpaceIds((current) => (
      enabled
        ? current.includes(spaceId) ? current : [...current, spaceId]
        : current.filter((id) => id !== spaceId)
    ));
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
  const embedSnippet = `<script type="text/javascript">
  (function () {
    window.helpin = window.helpin || function () {
      (window.helpinQ = window.helpinQ || []).push(arguments);
    };
    var t = document.createElement('script'),
        s = document.getElementsByTagName('script')[0];
    t.defer = true;
    t.id = 'helpin-widget';
    t.setAttribute('data-widget-key', '${widgetKey}');
    t.setAttribute('data-host', 'https://client.prod.helpin.ai');
    t.src = 'https://cdn.helpin.ai/lib.js';
    s.parentNode.insertBefore(t, s);
  })();
</script>`;
  const jsApiSnippet = `<script type="text/javascript">
  (function () {
    window.helpin = window.helpin || function () {
      (window.helpinQ = window.helpinQ || []).push(arguments);
    };
    var t = document.createElement('script'),
        s = document.getElementsByTagName('script')[0];
    t.defer = true;
    t.id = 'helpin-widget';
    t.setAttribute('data-widget-key', '${widgetKey}');
    t.setAttribute('data-host', 'https://client.prod.helpin.ai');
    t.src = 'https://cdn.helpin.ai/lib.js';
    s.parentNode.insertBefore(t, s);

    // Identify logged-in users (optional)
    helpin('boot', {
      key: '${widgetKey}',
      user: {
        email: 'user@example.com',
        name: 'Jane Doe',
        userId: 'your-internal-id'
      }
    });
  })();
</script>`;

  // Shared preview element used by both views
  const previewElement = (
    <WidgetPreview
      brandColor={brandColor}
      showBranding={showBranding}
      launcherPosition={launcherPosition}
      launcherIcon={launcherIcon}
      welcomeMessage={welcomeMessage}
      workspaceName={widgetName || workspace?.name}
      workspaceLogoUrl={widgetAvatarUrl || workspace?.logo_url}
      colorScheme={colorScheme}
      buttonColor={buttonColor}
      buttonIconColor={buttonIconColor}
      logoUrl={logoUrl}
    />
  );

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
            <CardDescription>Embed the chat widget on your website by adding the snippet to your HTML.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {!widgetKey ? (
              <div className="flex flex-col items-center gap-3 py-6 text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                  <KeyRound className="h-6 w-6 text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">No API key found</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Generate an API key to embed the chat widget on your website.
                  </p>
                </div>
                <Button
                  onClick={() => regenerateKeyMutation.mutate(undefined, {
                    onSuccess: () => toast.success('Widget API key generated'),
                    onError: (err: unknown) => toast.error(err instanceof Error ? err.message : 'Failed to generate key'),
                  })}
                  disabled={regenerateKeyMutation.isPending}
                  size="sm"
                >
                  {regenerateKeyMutation.isPending ? 'Generating...' : 'Generate API Key'}
                </Button>
              </div>
            ) : (
              <>
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
                  <div className="flex items-center gap-1 p-0.5 rounded-md border bg-muted/30 w-fit mb-2">
                    <button
                      onClick={() => setSnippetTab('basic')}
                      className={`px-2.5 py-1 rounded text-xs transition-colors ${
                        snippetTab === 'basic'
                          ? 'bg-background shadow-sm font-medium text-foreground'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      Basic
                    </button>
                    <button
                      onClick={() => setSnippetTab('advanced')}
                      className={`px-2.5 py-1 rounded text-xs transition-colors ${
                        snippetTab === 'advanced'
                          ? 'bg-background shadow-sm font-medium text-foreground'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      With User Identity
                    </button>
                  </div>
                  <CodeBlock
                    code={snippetTab === 'basic' ? embedSnippet : jsApiSnippet}
                    language="markup"
                    showLineNumbers
                  />
                  <p className="text-xs text-muted-foreground">
                    {snippetTab === 'basic'
                      ? 'Add this script tag before the closing </body> tag on every page where you want the widget.'
                      : 'Use this to identify logged-in users. Replace the placeholder values with real user data from your app.'}
                  </p>
                </div>
              </>
            )}
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
                <Label className="text-sm">Require phone number after email</Label>
                <p className="text-xs text-muted-foreground">Also ask for the visitor's phone number.</p>
              </div>
              <Switch checked={requirePhone} onCheckedChange={setRequirePhone} disabled={!requireEmail} />
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
            {/* Widget Identity */}
            <div className="space-y-3">
              <div>
                <Label className="text-sm font-medium">Widget Identity</Label>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Customize the name and avatar your customers see in the widget.
                </p>
              </div>
              <div className="flex items-start gap-4">
                {/* Avatar */}
                <div className="shrink-0">
                  {widgetAvatarUrl ? (
                    <div className="relative group">
                      <img
                        src={widgetAvatarUrl}
                        alt="Widget avatar"
                        className="h-14 w-14 rounded-full border object-cover cursor-pointer"
                        onClick={() => avatarInputRef.current?.click()}
                      />
                      <button
                        onClick={() => setWidgetAvatarUrl('')}
                        className="absolute -top-1 -right-1 h-5 w-5 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
                      >
                        &times;
                      </button>
                    </div>
                  ) : (
                    <div
                      className="h-14 w-14 rounded-full border-2 border-dashed flex items-center justify-center cursor-pointer hover:border-muted-foreground/50 transition-colors"
                      onClick={() => avatarInputRef.current?.click()}
                    >
                      <span className="text-lg font-semibold text-muted-foreground/50">
                        {(widgetName || workspace?.name || 'S').charAt(0).toUpperCase()}
                      </span>
                    </div>
                  )}
                  <input
                    ref={avatarInputRef}
                    type="file"
                    accept="image/png,image/jpeg,image/svg+xml"
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) {
                        if (!file.type.startsWith('image/')) {
                          toast.error('Please select an image file');
                          return;
                        }
                        if (file.size > 2 * 1024 * 1024) {
                          toast.error('Image must be under 2MB');
                          return;
                        }
                        const reader = new FileReader();
                        reader.onload = () => setWidgetAvatarUrl(reader.result as string);
                        reader.readAsDataURL(file);
                      }
                      e.target.value = '';
                    }}
                  />
                </div>
                {/* Name */}
                <div className="flex-1 space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Display Name</Label>
                  <Input
                    value={widgetName}
                    onChange={(e) => setWidgetName(e.target.value)}
                    placeholder={workspace?.name || 'Support'}
                    className="text-sm"
                  />
                  <p className="text-xs text-muted-foreground">
                    Shown in the widget header and conversations. Defaults to your workspace name.
                  </p>
                </div>
              </div>
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
                  className={`flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-6 transition-colors cursor-pointer ${
                    dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-muted-foreground/50'
                  }`}
                  onClick={() => logoInputRef.current?.click()}
                  onDragOver={handleDragOver}
                  onDragLeave={handleDragLeave}
                  onDrop={handleDrop}
                >
                  <ImageIcon className="h-6 w-6 text-muted-foreground/50" />
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
              <BrandColorPicker value={brandColor} onChange={setBrandColor} />
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
              <BrandColorPicker value={buttonColor} onChange={setButtonColor} />
            </div>

            {/* Button Icon Color */}
            <div className="space-y-2">
              <div>
                <Label className="text-sm font-medium">Button icon color</Label>
                <p className="text-xs text-muted-foreground mt-0.5">Icon color of the floating widget button.</p>
              </div>
              <BrandColorPicker value={buttonIconColor} onChange={setButtonIconColor} />
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

            {/* Show Powered By */}
            <div className="flex items-center justify-between pt-2">
              <div>
                <Label className="text-sm font-medium">Show "Powered by Helpin"</Label>
                <p className="text-xs text-muted-foreground mt-0.5">Display branding in the widget footer.</p>
              </div>
              <Switch checked={showBranding} onCheckedChange={setShowBranding} />
            </div>
          </CardContent>
        </Card>

        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <CardTitle className="text-base">Help Center</CardTitle>
            <CardDescription>Select which external docs spaces customers can browse from the widget Help tab.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {docsSpacesLoading && (
              <p className="text-sm text-muted-foreground">Loading available spaces...</p>
            )}

            {!docsSpacesLoading && externalDocsSpaces.length === 0 && (
              <p className="text-sm text-muted-foreground">
                Create an external space in Docs to show articles in the widget.
              </p>
            )}

            {!docsSpacesLoading && externalDocsSpaces.length > 0 && (
              <div className="space-y-3">
                {externalDocsSpaces.map((space) => {
                  const enabled = widgetHelpSpaceIds.includes(space.id);
                  return (
                    <div key={space.id} className="flex items-center justify-between gap-4 rounded-lg border p-4">
                      <div className="min-w-0">
                        <p className="text-sm font-medium">{space.name}</p>
                        <p className="text-xs text-muted-foreground">
                          /{space.slug}
                        </p>
                      </div>
                      <Switch
                        checked={enabled}
                        onCheckedChange={(checked) => toggleHelpSpace(space.id, checked)}
                      />
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </PreviewLayout>
  );
}
