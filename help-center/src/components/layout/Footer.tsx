import { useDocsContext } from '@/contexts/DocsContext'
import {
  Globe,
  Rss,
  type LucideIcon,
} from 'lucide-react'
import type { FooterSocialPlatform } from '@/lib/types'

const FONT_AWESOME_BRAND_ICON_BASE = 'https://d3gk2c5xim1je2.cloudfront.net/fontawesome/v7.2.0/brands'

type SocialPlatformMeta = {
  label: string
  icon?: LucideIcon
  brandIcon?: string
}

const socialPlatformMeta: Record<FooterSocialPlatform, SocialPlatformMeta> = {
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
}

function SocialPlatformIcon({
  platform,
  meta,
  className,
}: {
  platform: FooterSocialPlatform
  meta: SocialPlatformMeta
  className?: string
}) {
  if (meta.brandIcon) {
    const iconUrl = `${FONT_AWESOME_BRAND_ICON_BASE}/${meta.brandIcon}.svg`
    const normalizedPlatform = platform === 'twitter' ? 'x' : platform
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
    )
  }

  const Icon = meta.icon ?? Globe
  return <Icon aria-hidden="true" className={className} />
}

export function Footer() {
  const { config } = useDocsContext()
  const footerLinks = (config.footer_config?.links ?? []).filter((link) => link.label && link.url)
  const socialLinks = (config.footer_config?.social_links ?? []).filter((link) => link.platform && link.url)
  const showCopyright = config.footer_config?.show_copyright !== false
  const copyrightText = config.footer_config?.copyright_text
  const attributionSource = [
    config.subdomain || config.brand_name?.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
    config.workspace_id?.slice(0, 8),
  ].filter(Boolean).join('-')
  const attributionUrl = `https://helpin.ai/?utm_campaign=poweredBy&utm_medium=referral&utm_source=${encodeURIComponent(attributionSource || 'help-center')}`

  return (
    <footer className="mt-28 border-t border-border/70 pt-8 pb-24 text-[12px] text-muted-foreground/60 md:pb-28">
      <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <div
          data-testid="footer-text-links"
          className="flex min-w-0 max-w-3xl flex-wrap items-center gap-x-4 gap-y-2"
        >
          {showCopyright && (
            <span>{copyrightText || `\u00A9 ${new Date().getFullYear()} ${config.brand_name}`}</span>
          )}
          {footerLinks.map((link, i) => (
            <a
              key={i}
              href={link.url}
              target="_blank"
              rel="noopener noreferrer"
              className="transition-colors hover:text-foreground"
            >
              {link.label}
            </a>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-3 lg:justify-end">
          {socialLinks.length > 0 && (
            <div data-testid="footer-social-links" className="flex flex-wrap items-center gap-2">
              {socialLinks.map((link, i) => {
                const meta = socialPlatformMeta[link.platform] ?? socialPlatformMeta.website
                const label = link.label || meta.label

                return (
                  <a
                    key={`${link.platform}-${i}`}
                    href={link.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label={label}
                    title={label}
                    className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground/60 transition-colors hover:text-foreground"
                  >
                    <SocialPlatformIcon platform={link.platform} meta={meta} className="h-3.5 w-3.5" />
                  </a>
                )
              })}
            </div>
          )}

          <a
            href={attributionUrl}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Powered by Helpin"
            className="group inline-flex w-fit items-center gap-1 whitespace-nowrap text-muted-foreground/55 transition-colors hover:text-foreground"
          >
            <span>Powered by</span>
            <span className="inline-block font-medium text-muted-foreground/80 bg-[linear-gradient(currentColor,currentColor)] bg-[length:0_1px] bg-[position:0_100%] bg-no-repeat transition-[background-size] duration-200 ease-out group-hover:bg-[length:100%_1px]">
              Helpin
            </span>
          </a>
        </div>
      </div>
    </footer>
  )
}
