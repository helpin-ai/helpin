import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import {
  Plus, Trash2, GripVertical, ExternalLink, Info,
  Globe, Palette, Search, LayoutGrid, LinkIcon, ImageIcon,
} from 'lucide-react';
import { IconPicker } from '@/components/ui/icon-picker';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import type {
  HelpcenterHeaderLink,
  HelpcenterFooterLink,
  HelpcenterThemeMode,
  HomepageFeaturedCard,
  DocsSpace,
  DocsCollection,
} from '@/lib/docsTypes';

interface ConfigState {
  subdomain: string;
  custom_domain: string;
  brand_name: string;
  brand_logo_url: string;
  brand_color: string;
  favicon_url: string;
  theme_mode: HelpcenterThemeMode;
  header_links: HelpcenterHeaderLink[];
  footer_copyright_text: string;
  footer_links: HelpcenterFooterLink[];
  homepage_hero_title: string;
  homepage_hero_subtitle: string;
  homepage_featured_cards: HomepageFeaturedCard[];
  search_placeholder: string;
  is_published: boolean;
  seo_title: string;
  seo_description: string;
  support_email: string;
}

const DEFAULT_CONFIG: ConfigState = {
  subdomain: '',
  custom_domain: '',
  brand_name: '',
  brand_logo_url: '',
  brand_color: '#3b82f6',
  favicon_url: '',
  theme_mode: 'system',
  header_links: [],
  footer_copyright_text: '',
  footer_links: [],
  homepage_hero_title: '',
  homepage_hero_subtitle: '',
  homepage_featured_cards: [],
  search_placeholder: '',
  is_published: false,
  seo_title: '',
  seo_description: '',
  support_email: '',
};

export function HelpcenterTab({ workspaceId }: { workspaceId: string }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<ConfigState>(DEFAULT_CONFIG);
  const [spaces, setSpaces] = useState<DocsSpace[]>([]);
  const [homepageSpaceSlug, setHomepageSpaceSlug] = useState('');
  const [spaceCollections, setSpaceCollections] = useState<DocsCollection[]>([]);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      const { docsService } = await import('@/lib/services/docsService');
      const res = await docsService.getHelpcenterConfig(workspaceId);
      if (res.data) {
        const d = res.data;
        setConfig({
          subdomain: d.subdomain ?? '',
          custom_domain: d.custom_domain ?? '',
          brand_name: d.brand_name ?? '',
          brand_logo_url: d.brand_logo_url ?? '',
          brand_color: d.brand_color ?? '#3b82f6',
          favicon_url: d.favicon_url ?? '',
          theme_mode: d.theme_mode ?? 'system',
          header_links: d.header_links ?? [],
          footer_copyright_text: d.footer_config?.copyright_text ?? '',
          footer_links: d.footer_config?.links ?? [],
          homepage_hero_title: d.homepage_config?.hero_title ?? '',
          homepage_hero_subtitle: d.homepage_config?.hero_subtitle ?? '',
          homepage_featured_cards: d.homepage_config?.featured_cards ?? [],
          search_placeholder: d.search_placeholder ?? '',
          is_published: d.is_published ?? false,
          seo_title: d.seo_title ?? '',
          seo_description: d.seo_description ?? '',
          support_email: d.support_email ?? '',
        });
      }
      // Load external_capable spaces for featured card pickers
      const spacesRes = await docsService.listSpaces(workspaceId);
      const extSpaces = spacesRes.data?.filter(s => s.type === 'external_capable') ?? [];
      setSpaces(extSpaces);

      // If existing cards reference a space, auto-select it and sync icons from collections
      const existingSlug = res.data?.homepage_config?.featured_cards?.[0]?.space_slug;
      if (existingSlug) {
        const space = extSpaces.find(s => s.slug === existingSlug);
        if (space) {
          setHomepageSpaceSlug(existingSlug);
          const colRes = await docsService.listCollections(workspaceId, space.id);
          if (colRes.data) {
            setSpaceCollections(colRes.data);
            // Sync card icons from current collection data
            const existingCards = res.data?.homepage_config?.featured_cards ?? [];
            const synced = existingCards.map(card => {
              const col = colRes.data!.find(c => c.id === card.link_value);
              return col ? { ...card, icon: col.icon ?? '' } : card;
            });
            setConfig(prev => ({ ...prev, homepage_featured_cards: synced }));
          }
        }
      }

      setLoading(false);
    };
    load();
  }, [workspaceId]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.updateHelpcenterConfig(workspaceId, {
      subdomain: config.subdomain || undefined,
      custom_domain: config.custom_domain || undefined,
      brand_name: config.brand_name || undefined,
      brand_logo_url: config.brand_logo_url || undefined,
      brand_color: config.brand_color || undefined,
      favicon_url: config.favicon_url || undefined,
      theme_mode: config.theme_mode,
      header_links: config.header_links,
      footer_config: {
        copyright_text: config.footer_copyright_text,
        links: config.footer_links,
      },
      homepage_config: {
        hero_title: config.homepage_hero_title,
        hero_subtitle: config.homepage_hero_subtitle,
        featured_cards: config.homepage_featured_cards,
      },
      search_placeholder: config.search_placeholder || undefined,
      is_published: config.is_published,
      seo_title: config.seo_title || undefined,
      seo_description: config.seo_description || undefined,
      support_email: config.support_email || undefined,
    });
    // Sync icon changes back to collections
    for (const card of config.homepage_featured_cards) {
      if (card.link_type !== 'collection' || !card.link_value) continue;
      const col = spaceCollections.find(c => c.id === card.link_value);
      if (col && (col.icon ?? '') !== card.icon) {
        await docsService.updateCollection(workspaceId, col.id, { icon: card.icon });
      }
    }

    setSaving(false);
    if (res.error) {
      toast.error(res.error);
    } else {
      toast.success('Help center settings saved');
    }
  };

  // ── Header link helpers ──
  const addHeaderLink = () => {
    setConfig({
      ...config,
      header_links: [...config.header_links, { label: '', url: '', external: false }],
    });
  };

  const updateHeaderLink = (index: number, patch: Partial<HelpcenterHeaderLink>) => {
    const links = [...config.header_links];
    links[index] = { ...links[index], ...patch };
    setConfig({ ...config, header_links: links });
  };

  const removeHeaderLink = (index: number) => {
    setConfig({ ...config, header_links: config.header_links.filter((_, i) => i !== index) });
  };

  // ── Footer link helpers ──
  const addFooterLink = () => {
    setConfig({
      ...config,
      footer_links: [...config.footer_links, { label: '', url: '' }],
    });
  };

  const updateFooterLink = (index: number, patch: Partial<HelpcenterFooterLink>) => {
    const links = [...config.footer_links];
    links[index] = { ...links[index], ...patch };
    setConfig({ ...config, footer_links: links });
  };

  const removeFooterLink = (index: number) => {
    setConfig({ ...config, footer_links: config.footer_links.filter((_, i) => i !== index) });
  };

  // ── Featured card helpers (space-driven) ──
  const handleHomepageSpaceChange = async (slug: string) => {
    setHomepageSpaceSlug(slug);
    const space = spaces.find(s => s.slug === slug);
    if (!space) return;
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.listCollections(workspaceId, space.id);
    const cols = res.data ?? [];
    setSpaceCollections(cols);
    // Create a card for every collection
    const cards: HomepageFeaturedCard[] = cols.map(col => ({
      title: col.name,
      description: col.description ?? '',
      icon: col.icon ?? '',
      link_type: 'collection',
      link_value: col.id,
      space_slug: slug,
    }));
    setConfig(prev => ({ ...prev, homepage_featured_cards: cards }));
  };

  const toggleCollection = (colId: string) => {
    const exists = config.homepage_featured_cards.find(c => c.link_value === colId);
    if (exists) {
      setConfig(prev => ({
        ...prev,
        homepage_featured_cards: prev.homepage_featured_cards.filter(c => c.link_value !== colId),
      }));
    } else {
      const col = spaceCollections.find(c => c.id === colId);
      if (!col) return;
      setConfig(prev => ({
        ...prev,
        homepage_featured_cards: [...prev.homepage_featured_cards, {
          title: col.name,
          description: col.description ?? '',
          icon: col.icon ?? '',
          link_type: 'collection',
          link_value: col.id,
          space_slug: homepageSpaceSlug,
        }],
      }));
    }
  };

  const updateCardByCollectionId = (colId: string, patch: Partial<HomepageFeaturedCard>) => {
    setConfig(prev => ({
      ...prev,
      homepage_featured_cards: prev.homepage_featured_cards.map(c =>
        c.link_value === colId ? { ...c, ...patch } : c
      ),
    }));
  };

  // ── Asset upload helpers ──
  const logoInputRef = useRef<HTMLInputElement>(null);
  const faviconInputRef = useRef<HTMLInputElement>(null);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const [uploadingFavicon, setUploadingFavicon] = useState(false);

  const handleAssetUpload = async (
    e: ChangeEvent<HTMLInputElement>,
    assetType: 'logo' | 'favicon',
    setUploading: (v: boolean) => void,
    field: 'brand_logo_url' | 'favicon_url',
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2 MB');
      return;
    }
    setUploading(true);
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.uploadHelpcenterAsset(workspaceId, assetType, file);
    setUploading(false);
    e.target.value = '';
    if (res.error || !res.data) {
      toast.error(res.error ?? 'Upload failed');
      return;
    }
    setConfig((prev) => ({ ...prev, [field]: res.data!.url }));
    toast.success(`${assetType === 'logo' ? 'Logo' : 'Favicon'} uploaded`);
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-16 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-32 w-full rounded-lg" />
      </div>
    );
  }

  return (
    <form onSubmit={handleSave} className="space-y-8">
      {/* ── Top Actions ── */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium">Help Center Configuration</h3>
          <p className="text-xs text-muted-foreground mt-0.5">Manage your public help center settings.</p>
        </div>
        <Button type="submit" disabled={saving} size="sm">
          {saving ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      {/* ── Publish Status Bar ── */}
      <div className="flex items-center gap-4 rounded-lg border bg-card p-4">
        <Switch
          checked={config.is_published}
          onCheckedChange={(v) => setConfig({ ...config, is_published: v })}
        />
        <div>
          <div className="flex items-center gap-2">
            <p className="text-sm font-medium">Help Center</p>
            <Badge variant={config.is_published ? 'default' : 'secondary'} className="text-[11px] px-1.5 py-0">
              {config.is_published ? 'Live' : 'Offline'}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            {config.is_published
              ? 'Your help center is publicly accessible.'
              : 'Toggle to make your help center visible to the public.'}
          </p>
        </div>
      </div>

      {/* ── Branding ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <Palette className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Branding</CardTitle>
          </div>
          <CardDescription>Customize your help center&apos;s visual identity.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Upload zones side by side */}
          <div className="grid gap-6 sm:grid-cols-2">
            {/* Logo upload zone */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Logo</Label>
              {config.brand_logo_url ? (
                <div className="group relative flex h-28 items-center justify-center rounded-lg border-2 border-dashed bg-muted/30 transition-colors hover:bg-muted/50">
                  <img src={config.brand_logo_url} alt="Logo" className="max-h-16 max-w-[160px] object-contain" />
                  <div className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-background/80 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button type="button" variant="outline" size="sm" disabled={uploadingLogo} onClick={() => logoInputRef.current?.click()}>
                      {uploadingLogo ? 'Uploading...' : 'Replace'}
                    </Button>
                    <Button type="button" variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setConfig({ ...config, brand_logo_url: '' })}>
                      Remove
                    </Button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  disabled={uploadingLogo}
                  onClick={() => logoInputRef.current?.click()}
                  className="flex h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed bg-muted/20 text-muted-foreground transition-colors hover:border-primary/30 hover:bg-muted/40 hover:text-foreground"
                >
                  <ImageIcon className="h-6 w-6" />
                  <span className="text-xs">{uploadingLogo ? 'Uploading...' : '200 × 50 px · SVG or PNG'}</span>
                </button>
              )}
              <input ref={logoInputRef} type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" className="hidden" onChange={(e) => handleAssetUpload(e, 'logo', setUploadingLogo, 'brand_logo_url')} />
            </div>
            {/* Favicon upload zone */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Favicon</Label>
              {config.favicon_url ? (
                <div className="group relative flex h-28 items-center justify-center rounded-lg border-2 border-dashed bg-muted/30 transition-colors hover:bg-muted/50">
                  <img src={config.favicon_url} alt="Favicon" className="h-10 w-10 object-contain" />
                  <div className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-background/80 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button type="button" variant="outline" size="sm" disabled={uploadingFavicon} onClick={() => faviconInputRef.current?.click()}>
                      {uploadingFavicon ? 'Uploading...' : 'Replace'}
                    </Button>
                    <Button type="button" variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setConfig({ ...config, favicon_url: '' })}>
                      Remove
                    </Button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  disabled={uploadingFavicon}
                  onClick={() => faviconInputRef.current?.click()}
                  className="flex h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed bg-muted/20 text-muted-foreground transition-colors hover:border-primary/30 hover:bg-muted/40 hover:text-foreground"
                >
                  <ImageIcon className="h-5 w-5" />
                  <span className="text-xs">{uploadingFavicon ? 'Uploading...' : '32 × 32 px · ICO, PNG, or SVG'}</span>
                </button>
              )}
              <input ref={faviconInputRef} type="file" accept="image/png,image/jpeg,image/svg+xml,image/x-icon,image/vnd.microsoft.icon" className="hidden" onChange={(e) => handleAssetUpload(e, 'favicon', setUploadingFavicon, 'favicon_url')} />
            </div>
          </div>

          <Separator />

          {/* Brand Name + Color + Theme */}
          <div className="grid gap-5 sm:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="hc-brand-name">Brand Name</Label>
              <Input
                id="hc-brand-name"
                value={config.brand_name}
                onChange={(e) => setConfig({ ...config, brand_name: e.target.value })}
                placeholder="Your Company"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-brand-color">Brand Color</Label>
              <div className="flex items-center gap-2">
                <div className="relative shrink-0">
                  <div
                    className="h-9 w-9 rounded-md border shadow-sm cursor-pointer"
                    style={{ backgroundColor: config.brand_color }}
                  />
                  <input
                    type="color"
                    id="hc-brand-color"
                    value={config.brand_color}
                    onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                    className="absolute inset-0 cursor-pointer opacity-0"
                  />
                </div>
                <Input
                  value={config.brand_color}
                  onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                  className="flex-1 font-mono text-sm"
                  placeholder="#3b82f6"
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex items-center gap-1.5">
                <Label htmlFor="hc-theme-mode">Theme Mode</Label>
                <TooltipProvider delayDuration={200}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Info className="h-3.5 w-3.5 text-muted-foreground cursor-help" />
                    </TooltipTrigger>
                    <TooltipContent side="top" className="max-w-[260px] text-xs leading-relaxed">
                      <p><strong>Light</strong> — Forces light theme, hides toggle</p>
                      <p><strong>Dark</strong> — Forces dark theme, hides toggle</p>
                      <p><strong>System</strong> — Follows visitor&apos;s OS, shows toggle</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <Select
                value={config.theme_mode}
                onValueChange={(v) => setConfig({ ...config, theme_mode: v as HelpcenterThemeMode })}
              >
                <SelectTrigger id="hc-theme-mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="light">Light</SelectItem>
                  <SelectItem value="dark">Dark</SelectItem>
                  <SelectItem value="system">System (auto)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Domain & SEO ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <Globe className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Domain</CardTitle>
            </div>
            <CardDescription>Configure your help center URL.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-subdomain">Subdomain</Label>
              <div className="flex items-center">
                <Input
                  id="hc-subdomain"
                  value={config.subdomain}
                  onChange={(e) => setConfig({ ...config, subdomain: e.target.value })}
                  placeholder="yourcompany"
                  className="rounded-r-none border-r-0"
                />
                <span className="flex h-9 shrink-0 items-center rounded-r-md border bg-muted/50 px-3 text-sm text-muted-foreground">.helpin.ai</span>
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-custom-domain">Custom Domain</Label>
              <Input
                id="hc-custom-domain"
                value={config.custom_domain}
                onChange={(e) => setConfig({ ...config, custom_domain: e.target.value })}
                placeholder="help.yourcompany.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-support-email">Support Email</Label>
              <Input
                id="hc-support-email"
                type="email"
                value={config.support_email}
                onChange={(e) => setConfig({ ...config, support_email: e.target.value })}
                placeholder="support@yourcompany.com"
              />
            </div>
          </CardContent>
        </Card>

        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <Search className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">SEO</CardTitle>
            </div>
            <CardDescription>Optimize for search engines.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-seo-title">Meta Title</Label>
              <Input
                id="hc-seo-title"
                value={config.seo_title}
                onChange={(e) => setConfig({ ...config, seo_title: e.target.value })}
                placeholder="Help Center - Your Company"
              />
              <p className="text-[11px] text-muted-foreground">{config.seo_title.length}/60 characters</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-seo-desc">Meta Description</Label>
              <Textarea
                id="hc-seo-desc"
                value={config.seo_description}
                onChange={(e) => setConfig({ ...config, seo_description: e.target.value })}
                placeholder="Find answers, guides, and documentation..."
                rows={3}
              />
              <p className="text-[11px] text-muted-foreground">{config.seo_description.length}/160 characters</p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ── Homepage ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <LayoutGrid className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Homepage</CardTitle>
          </div>
          <CardDescription>Configure the hero section and featured content visitors see first.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="hc-hero-title">Hero Title</Label>
              <Input
                id="hc-hero-title"
                value={config.homepage_hero_title}
                onChange={(e) => setConfig({ ...config, homepage_hero_title: e.target.value })}
                placeholder="How can we help?"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-search-placeholder">Search Placeholder</Label>
              <Input
                id="hc-search-placeholder"
                value={config.search_placeholder}
                onChange={(e) => setConfig({ ...config, search_placeholder: e.target.value })}
                placeholder="Search articles..."
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="hc-hero-subtitle">Hero Subtitle</Label>
            <Input
              id="hc-hero-subtitle"
              value={config.homepage_hero_subtitle}
              onChange={(e) => setConfig({ ...config, homepage_hero_subtitle: e.target.value })}
              placeholder="Search our knowledge base or browse topics below"
            />
          </div>

          <Separator />

          {/* Featured Cards */}
          <div className="space-y-3">
            <div>
              <Label className="text-sm">Featured Cards</Label>
              <p className="text-xs text-muted-foreground mt-0.5">
                Select a space to populate homepage cards from its collections.
              </p>
            </div>
            <Select value={homepageSpaceSlug} onValueChange={handleHomepageSpaceChange}>
              <SelectTrigger className="w-full sm:w-64">
                <SelectValue placeholder="Select a space..." />
              </SelectTrigger>
              <SelectContent>
                {spaces.map(s => (
                  <SelectItem key={s.id} value={s.slug}>
                    {s.icon ? `${s.icon} ${s.name}` : s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {spaceCollections.length > 0 && (
              <div className="space-y-1.5">
                {spaceCollections.map(col => {
                  const card = config.homepage_featured_cards.find(c => c.link_value === col.id);
                  const checked = !!card;
                  return (
                    <div
                      key={col.id}
                      className={`flex items-center gap-3 rounded-lg border p-3 transition-colors ${checked ? 'border-primary/20 bg-primary/[0.03]' : 'border-transparent bg-muted/30'}`}
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={() => toggleCollection(col.id)}
                      />
                      {checked ? (
                        <div className="flex-1 grid gap-2 grid-cols-[40px_minmax(0,160px)_1fr]">
                          <IconPicker
                            value={card.icon}
                            onChange={(v) => updateCardByCollectionId(col.id, { icon: v })}
                          />
                          <Input
                            value={card.title}
                            onChange={(e) => updateCardByCollectionId(col.id, { title: e.target.value })}
                            placeholder="Card title"
                            className="h-8 text-sm"
                          />
                          <Input
                            value={card.description}
                            onChange={(e) => updateCardByCollectionId(col.id, { description: e.target.value })}
                            placeholder="Short description"
                            className="h-8 text-sm"
                          />
                        </div>
                      ) : (
                        <span className="text-sm text-muted-foreground">{col.name}</span>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* ── Navigation ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <LinkIcon className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Header Links</CardTitle>
            </div>
            <CardDescription>Navigation links displayed in the top bar.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {config.header_links.length === 0 && (
              <p className="text-xs text-muted-foreground py-3 text-center">No header links yet. Add one below.</p>
            )}
            {config.header_links.map((link, i) => (
              <div key={i} className="flex items-center gap-2 group">
                <GripVertical className="h-4 w-4 shrink-0 text-muted-foreground/50" />
                <Input
                  value={link.label}
                  onChange={(e) => updateHeaderLink(i, { label: e.target.value })}
                  placeholder="Label"
                  className="w-28 h-8 text-sm"
                />
                <Input
                  value={link.url}
                  onChange={(e) => updateHeaderLink(i, { url: e.target.value })}
                  placeholder="https://..."
                  className="flex-1 h-8 text-sm"
                />
                <TooltipProvider delayDuration={200}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className={`h-8 w-8 shrink-0 ${link.external ? 'text-primary' : 'text-muted-foreground/50'}`}
                        onClick={() => updateHeaderLink(i, { external: !link.external })}
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent side="top" className="text-xs">
                      {link.external ? 'Opens in new tab' : 'Opens in same tab'}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => removeHeaderLink(i)}>
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            ))}
            <Button type="button" variant="outline" size="sm" className="h-8 text-xs" onClick={addHeaderLink}>
              <Plus className="mr-1 h-3.5 w-3.5" /> Add Link
            </Button>
          </CardContent>
        </Card>

        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <LinkIcon className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Footer</CardTitle>
            </div>
            <CardDescription>Copyright text and footer navigation.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-footer-copyright" className="text-sm">Copyright Text</Label>
              <Input
                id="hc-footer-copyright"
                value={config.footer_copyright_text}
                onChange={(e) => setConfig({ ...config, footer_copyright_text: e.target.value })}
                placeholder={`\u00A9 ${new Date().getFullYear()} Your Company. All rights reserved.`}
              />
            </div>
            <Separator />
            <div className="space-y-3">
              <Label className="text-sm">Footer Links</Label>
              {config.footer_links.length === 0 && (
                <p className="text-xs text-muted-foreground py-2 text-center">No footer links yet.</p>
              )}
              {config.footer_links.map((link, i) => (
                <div key={i} className="flex items-center gap-2 group">
                  <Input
                    value={link.label}
                    onChange={(e) => updateFooterLink(i, { label: e.target.value })}
                    placeholder="Label"
                    className="w-28 h-8 text-sm"
                  />
                  <Input
                    value={link.url}
                    onChange={(e) => updateFooterLink(i, { url: e.target.value })}
                    placeholder="https://..."
                    className="flex-1 h-8 text-sm"
                  />
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => removeFooterLink(i)}>
                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </div>
              ))}
              <Button type="button" variant="outline" size="sm" className="h-8 text-xs" onClick={addFooterLink}>
                <Plus className="mr-1 h-3.5 w-3.5" /> Add Link
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

    </form>
  );
}
