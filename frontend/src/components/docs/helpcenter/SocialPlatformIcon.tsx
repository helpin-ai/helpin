import { Globe, Rss, type LucideIcon } from 'lucide-react';
import type { HelpcenterSocialPlatform } from '@/lib/docsTypes';

const FONT_AWESOME_BRAND_ICON_BASE = 'https://d3gk2c5xim1je2.cloudfront.net/fontawesome/v7.2.0/brands';

export type SocialPlatformMeta = {
  label: string;
  icon?: LucideIcon;
  brandIcon?: string;
};

export const SOCIAL_PLATFORM_META: Record<HelpcenterSocialPlatform, SocialPlatformMeta> = {
  x: { label: 'X', brandIcon: 'x-twitter' },
  twitter: { label: 'X', brandIcon: 'x-twitter' },
  linkedin: { label: 'LinkedIn', brandIcon: 'linkedin' },
  github: { label: 'GitHub', brandIcon: 'github' },
  youtube: { label: 'YouTube', brandIcon: 'youtube' },
  facebook: { label: 'Facebook', brandIcon: 'facebook' },
  instagram: { label: 'Instagram', brandIcon: 'instagram' },
  discord: { label: 'Discord', brandIcon: 'discord' },
  slack: { label: 'Slack', brandIcon: 'slack' },
  rss: { label: 'RSS', icon: Rss },
  website: { label: 'Website', icon: Globe },
};

export function SocialPlatformIcon({
  platform,
  meta = SOCIAL_PLATFORM_META[platform],
  className,
}: {
  platform: HelpcenterSocialPlatform;
  meta?: SocialPlatformMeta;
  className?: string;
}) {
  if (meta?.brandIcon) {
    const iconUrl = `${FONT_AWESOME_BRAND_ICON_BASE}/${meta.brandIcon}.svg`;
    const normalizedPlatform = platform === 'twitter' ? 'x' : platform;
    return (
      <span
        aria-hidden="true"
        data-social-brand-icon={normalizedPlatform}
        className={`inline-block bg-current ${className ?? ''}`}
        style={{
          WebkitMaskImage: `url(${iconUrl})`,
          WebkitMaskRepeat: 'no-repeat',
          WebkitMaskPosition: 'center',
          WebkitMaskSize: 'contain',
          maskImage: `url(${iconUrl})`,
          maskRepeat: 'no-repeat',
          maskPosition: 'center',
          maskSize: 'contain',
        }}
      />
    );
  }

  const Icon = meta?.icon ?? Globe;
  return <Icon aria-hidden="true" className={className} />;
}
