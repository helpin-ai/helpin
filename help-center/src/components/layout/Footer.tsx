import { useDocsContext } from '@/contexts/DocsContext'

export function Footer() {
  const { config } = useDocsContext()

  return (
    <footer className="border-t border-border py-5 px-6 flex items-center justify-between text-[12px] text-muted-foreground/60">
      <span>
        &copy; {new Date().getFullYear()} {config.brand_name}
      </span>
      <div className="flex items-center gap-4">
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
          <span className="font-medium text-muted-foreground/80">Helpin</span>
        </span>
      </div>
    </footer>
  )
}
