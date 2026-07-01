import { useDocsContext } from '@/contexts/DocsContext'

export function Footer() {
  const { config } = useDocsContext()
  const footerLinks = config.footer_config?.links ?? []
  const copyrightText = config.footer_config?.copyright_text
  const attributionSource = [
    config.subdomain || config.brand_name?.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
    config.workspace_id?.slice(0, 8),
  ].filter(Boolean).join('-')
  const attributionUrl = `https://helpin.ai/?utm_campaign=poweredBy&utm_medium=referral&utm_source=${encodeURIComponent(attributionSource || 'help-center')}`

  return (
    <footer className="mt-28 border-t border-border/70 pt-8 pb-24 text-[12px] text-muted-foreground/60 md:pb-28">
      <div className="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <span>
            {copyrightText || `\u00A9 ${new Date().getFullYear()} ${config.brand_name}`}
          </span>
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
    </footer>
  )
}
