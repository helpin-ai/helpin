import { useDocsContext } from '@/contexts/DocsContext'

export function Footer() {
  const { config } = useDocsContext()
  const footerLinks = config.footer_config?.links ?? []
  const copyrightText = config.footer_config?.copyright_text

  return (
    <footer className="border-t border-border py-5 px-6 flex items-center justify-between text-[12px] text-muted-foreground/60">
      <span>
        {copyrightText || `\u00A9 ${new Date().getFullYear()} ${config.brand_name}`}
      </span>
      <div className="flex items-center gap-4">
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
        <span>
          Powered by{' '}
          <a href="https://helpin.ai" target="_blank" rel="noopener noreferrer" className="font-medium text-muted-foreground/80 transition-colors hover:text-foreground">Helpin</a>
        </span>
      </div>
    </footer>
  )
}
