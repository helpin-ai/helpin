import { useDocsContext } from '@/contexts/DocsContext'
import {
  Disc,
  Facebook,
  Github,
  Globe,
  Instagram,
  Linkedin,
  Rss,
  Slack,
  Twitter,
  Youtube,
  type LucideIcon,
} from 'lucide-react'
import type { FooterSocialPlatform } from '@/lib/types'

const socialPlatformMeta: Record<FooterSocialPlatform, { label: string; icon: LucideIcon }> = {
  x: { label: 'X', icon: Twitter },
  twitter: { label: 'X', icon: Twitter },
  linkedin: { label: 'LinkedIn', icon: Linkedin },
  github: { label: 'GitHub', icon: Github },
  youtube: { label: 'YouTube', icon: Youtube },
  facebook: { label: 'Facebook', icon: Facebook },
  instagram: { label: 'Instagram', icon: Instagram },
  discord: { label: 'Discord', icon: Disc },
  slack: { label: 'Slack', icon: Slack },
  rss: { label: 'RSS', icon: Rss },
  website: { label: 'Website', icon: Globe },
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
          {config.support_email && (
            <a
              href={`mailto:${config.support_email}`}
              className="transition-colors hover:text-foreground"
            >
              Contact support
            </a>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-3 lg:justify-end">
          {socialLinks.length > 0 && (
            <div data-testid="footer-social-links" className="flex flex-wrap items-center gap-2">
              {socialLinks.map((link, i) => {
                const meta = socialPlatformMeta[link.platform] ?? socialPlatformMeta.website
                const Icon = meta.icon
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
                    <Icon aria-hidden="true" className="h-3.5 w-3.5" />
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
