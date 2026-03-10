import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import {
  Plus, Trash2, GripVertical, ExternalLink, Upload, Info,
} from 'lucide-react';
import { IconPicker } from '@/components/ui/icon-picker';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
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
      <div className="space-y-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  return (
    <form onSubmit={handleSave} className="space-y-6">
      {/* ── Top bar: Publishing + Save ── */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Switch
            checked={config.is_published}
            onCheckedChange={(v) => setConfig({ ...config, is_published: v })}
          />
          <div>
            <p className="text-sm font-medium">Help Center Published</p>
            <p className="text-xs text-muted-foreground">
              {config.is_published
                ? 'Your help center is publicly accessible.'
                : 'Your help center is not visible to the public.'}
            </p>
          </div>
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>

      {/* ── Branding ── */}
      <Card>
        <CardHeader>
          <CardTitle>Branding</CardTitle>
          <CardDescription>Customize how your public help center looks.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {/* Row 1: Logo + Favicon side by side */}
          <div className="grid gap-5 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Logo</Label>
              <p className="text-xs text-muted-foreground">
                Recommended: 200 &times; 50 px (SVG or PNG). Max 2 MB.
              </p>
              {config.brand_logo_url ? (
                <div className="flex items-center gap-3">
                  <div className="flex h-14 w-28 items-center justify-center rounded-md border bg-muted/40 p-1.5">
                    <img src={config.brand_logo_url} alt="Logo preview" className="max-h-full max-w-full object-contain" />
                  </div>
                  <Button type="button" variant="outline" size="sm" disabled={uploadingLogo} onClick={() => logoInputRef.current?.click()}>
                    <Upload className="mr-1.5 h-3.5 w-3.5" />
                    {uploadingLogo ? 'Uploading...' : 'Replace'}
                  </Button>
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => setConfig({ ...config, brand_logo_url: '' })} title="Remove logo">
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ) : (
                <Button type="button" variant="outline" size="sm" disabled={uploadingLogo} onClick={() => logoInputRef.current?.click()}>
                  <Upload className="mr-1.5 h-4 w-4" />
                  {uploadingLogo ? 'Uploading...' : 'Upload Logo'}
                </Button>
              )}
              <input ref={logoInputRef} type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" className="hidden" onChange={(e) => handleAssetUpload(e, 'logo', setUploadingLogo, 'brand_logo_url')} />
            </div>
            <div className="space-y-2">
              <Label>Favicon</Label>
              <p className="text-xs text-muted-foreground">
                Recommended: 32 &times; 32 px (ICO, PNG, or SVG). Max 2 MB.
              </p>
              {config.favicon_url ? (
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-md border bg-muted/40 p-1">
                    <img src={config.favicon_url} alt="Favicon preview" className="max-h-full max-w-full object-contain" />
                  </div>
                  <Button type="button" variant="outline" size="sm" disabled={uploadingFavicon} onClick={() => faviconInputRef.current?.click()}>
                    <Upload className="mr-1.5 h-3.5 w-3.5" />
                    {uploadingFavicon ? 'Uploading...' : 'Replace'}
                  </Button>
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => setConfig({ ...config, favicon_url: '' })} title="Remove favicon">
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ) : (
                <Button type="button" variant="outline" size="sm" disabled={uploadingFavicon} onClick={() => faviconInputRef.current?.click()}>
                  <Upload className="mr-1.5 h-4 w-4" />
                  {uploadingFavicon ? 'Uploading...' : 'Upload Favicon'}
                </Button>
              )}
              <input ref={faviconInputRef} type="file" accept="image/png,image/jpeg,image/svg+xml,image/x-icon,image/vnd.microsoft.icon" className="hidden" onChange={(e) => handleAssetUpload(e, 'favicon', setUploadingFavicon, 'favicon_url')} />
            </div>
          </div>
          {/* Row 2: Brand Name + Color + Theme */}
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
                <input
                  type="color"
                  id="hc-brand-color"
                  value={config.brand_color}
                  onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                  className="h-9 w-10 cursor-pointer rounded border"
                />
                <Input
                  value={config.brand_color}
                  onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                  className="flex-1"
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
                      <p><strong>System</strong> — Follows visitor&apos;s OS preference, shows a toggle so they can switch</p>
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
                  <SelectItem value="system">System (visitor preference)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Domain & SEO (side by side) ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Domain</CardTitle>
            <CardDescription>Set up your help center URL.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-subdomain">Subdomain</Label>
              <div className="flex items-center gap-1">
                <Input
                  id="hc-subdomain"
                  value={config.subdomain}
                  onChange={(e) => setConfig({ ...config, subdomain: e.target.value })}
                  placeholder="yourcompany"
                />
                <span className="shrink-0 text-sm text-muted-foreground">.helpin.ai</span>
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

        <Card>
          <CardHeader>
            <CardTitle>SEO</CardTitle>
            <CardDescription>Optimize your help center for search engines.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-seo-title">SEO Title</Label>
              <Input
                id="hc-seo-title"
                value={config.seo_title}
                onChange={(e) => setConfig({ ...config, seo_title: e.target.value })}
                placeholder="Help Center - Your Company"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-seo-desc">SEO Description</Label>
              <Textarea
                id="hc-seo-desc"
                value={config.seo_description}
                onChange={(e) => setConfig({ ...config, seo_description: e.target.value })}
                placeholder="Find answers, guides, and documentation..."
                rows={3}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ── Homepage ── */}
      <Card>
        <CardHeader>
          <CardTitle>Homepage</CardTitle>
          <CardDescription>Configure the hero section visitors see first.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
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

          {/* Featured Cards */}
          <div className="space-y-3 pt-2">
            <div>
              <Label>Featured Cards</Label>
              <p className="text-xs text-muted-foreground mt-0.5">
                Select a space to populate homepage cards from its collections. Uncheck any you don't want to show.
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
              <div className="space-y-2">
                {spaceCollections.map(col => {
                  const card = config.homepage_featured_cards.find(c => c.link_value === col.id);
                  const checked = !!card;
                  return (
                    <div key={col.id} className="flex items-center gap-2 rounded-lg border p-3">
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
                          />
                          <Input
                            value={card.description}
                            onChange={(e) => updateCardByCollectionId(col.id, { description: e.target.value })}
                            placeholder="Short description"
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

      {/* ── Header & Footer Links (side by side) ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Header Links</CardTitle>
            <CardDescription>Navigation links in the top bar.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {config.header_links.map((link, i) => (
              <div key={i} className="flex items-center gap-2">
                <GripVertical className="h-4 w-4 shrink-0 text-muted-foreground" />
                <Input
                  value={link.label}
                  onChange={(e) => updateHeaderLink(i, { label: e.target.value })}
                  placeholder="Label"
                  className="w-24"
                />
                <Input
                  value={link.url}
                  onChange={(e) => updateHeaderLink(i, { url: e.target.value })}
                  placeholder="https://..."
                  className="flex-1"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={`shrink-0 ${link.external ? 'text-primary' : 'text-muted-foreground'}`}
                  onClick={() => updateHeaderLink(i, { external: !link.external })}
                  title={link.external ? 'Opens in new tab' : 'Opens in same tab'}
                >
                  <ExternalLink className="h-4 w-4" />
                </Button>
                <Button type="button" variant="ghost" size="icon" className="shrink-0" onClick={() => removeHeaderLink(i)}>
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={addHeaderLink}>
              <Plus className="mr-1 h-4 w-4" /> Add Link
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Footer</CardTitle>
            <CardDescription>Copyright and footer links.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-footer-copyright">Copyright Text</Label>
              <Input
                id="hc-footer-copyright"
                value={config.footer_copyright_text}
                onChange={(e) => setConfig({ ...config, footer_copyright_text: e.target.value })}
                placeholder={`\u00A9 ${new Date().getFullYear()} Your Company. All rights reserved.`}
              />
            </div>
            <div className="space-y-3">
              <Label>Footer Links</Label>
              {config.footer_links.map((link, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Input
                    value={link.label}
                    onChange={(e) => updateFooterLink(i, { label: e.target.value })}
                    placeholder="Label"
                    className="w-24"
                  />
                  <Input
                    value={link.url}
                    onChange={(e) => updateFooterLink(i, { url: e.target.value })}
                    placeholder="https://..."
                    className="flex-1"
                  />
                  <Button type="button" variant="ghost" size="icon" className="shrink-0" onClick={() => removeFooterLink(i)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              ))}
              <Button type="button" variant="outline" size="sm" onClick={addFooterLink}>
                <Plus className="mr-1 h-4 w-4" /> Add Link
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

    </form>
  );
}
