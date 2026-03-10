import { useEffect, useState, type FormEvent } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { toast } from 'sonner';

export function HelpcenterTab({ workspaceId }: { workspaceId: string }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<{
    subdomain: string;
    custom_domain: string;
    brand_name: string;
    brand_logo_url: string;
    brand_color: string;
    is_published: boolean;
    seo_title: string;
    seo_description: string;
    support_email: string;
  }>({
    subdomain: '',
    custom_domain: '',
    brand_name: '',
    brand_logo_url: '',
    brand_color: '#3b82f6',
    is_published: false,
    seo_title: '',
    seo_description: '',
    support_email: '',
  });

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      const { docsService } = await import('@/lib/services/docsService');
      const res = await docsService.getHelpcenterConfig(workspaceId);
      if (res.data) {
        setConfig({
          subdomain: res.data.subdomain ?? '',
          custom_domain: res.data.custom_domain ?? '',
          brand_name: res.data.brand_name ?? '',
          brand_logo_url: res.data.brand_logo_url ?? '',
          brand_color: res.data.brand_color ?? '#3b82f6',
          is_published: res.data.is_published ?? false,
          seo_title: res.data.seo_title ?? '',
          seo_description: res.data.seo_description ?? '',
          support_email: res.data.support_email ?? '',
        });
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
      is_published: config.is_published,
      seo_title: config.seo_title || undefined,
      seo_description: config.seo_description || undefined,
      support_email: config.support_email || undefined,
    });
    setSaving(false);
    if (res.error) {
      toast.error(res.error);
    } else {
      toast.success('Help center settings saved');
    }
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
      <Card>
        <CardHeader>
          <CardTitle>Branding</CardTitle>
          <CardDescription>Customize how your public help center looks.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-name">Brand Name</Label>
            <Input
              id="hc-brand-name"
              value={config.brand_name}
              onChange={(e) => setConfig({ ...config, brand_name: e.target.value })}
              placeholder="Your Company"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-logo">Logo URL</Label>
            <Input
              id="hc-brand-logo"
              value={config.brand_logo_url}
              onChange={(e) => setConfig({ ...config, brand_logo_url: e.target.value })}
              placeholder="https://..."
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-color">Brand Color</Label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                id="hc-brand-color"
                value={config.brand_color}
                onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                className="h-8 w-12 cursor-pointer rounded border"
              />
              <Input
                value={config.brand_color}
                onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                className="flex-1"
                placeholder="#3b82f6"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Domain</CardTitle>
          <CardDescription>Set up your help center URL.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
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
          <div className="grid gap-2">
            <Label htmlFor="hc-custom-domain">Custom Domain (optional)</Label>
            <Input
              id="hc-custom-domain"
              value={config.custom_domain}
              onChange={(e) => setConfig({ ...config, custom_domain: e.target.value })}
              placeholder="help.yourcompany.com"
            />
          </div>
          <div className="grid gap-2">
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
          <div className="grid gap-2">
            <Label htmlFor="hc-seo-title">SEO Title</Label>
            <Input
              id="hc-seo-title"
              value={config.seo_title}
              onChange={(e) => setConfig({ ...config, seo_title: e.target.value })}
              placeholder="Help Center - Your Company"
            />
          </div>
          <div className="grid gap-2">
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

      <Card>
        <CardHeader>
          <CardTitle>Publishing</CardTitle>
          <CardDescription>Control whether your help center is live.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Help Center Published</p>
              <p className="text-xs text-muted-foreground">
                {config.is_published
                  ? 'Your help center is publicly accessible.'
                  : 'Your help center is not visible to the public.'}
              </p>
            </div>
            <Switch
              checked={config.is_published}
              onCheckedChange={(v) => setConfig({ ...config, is_published: v })}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </form>
  );
}
