import Link from 'next/link';

const FOOTER_LINKS = {
  Legal: [
    { label: 'Privacy Policy', href: '/privacy' },
    { label: 'Terms of Service', href: '/terms' },
  ],
};

export function Footer() {
  return (
    <footer className="border-t border-border/40 bg-muted/20">
      <div className="mx-auto max-w-7xl px-6 py-16 lg:px-8">
        <div className="flex flex-col sm:flex-row justify-between gap-10">
          <div>
            <Link href="/">
              <img src="https://assets.helpin.ai/logos/helpin-light-mode-logo.svg" alt="Helpin" className="h-6" />
            </Link>
            <p className="mt-4 text-sm leading-relaxed text-muted-foreground max-w-xs">
              PM, CRM, support & docs — connected
              by AI agents that do the work.
            </p>
          </div>

          {Object.entries(FOOTER_LINKS).map(([heading, links]) => (
            <div key={heading} className="text-left sm:text-right">
              <h3 className="text-sm font-bold tracking-wide">{heading}</h3>
              <ul className="mt-4 space-y-3">
                {links.map((link) => (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-14 border-t border-border/40 pt-6 flex flex-wrap items-center justify-between gap-4 text-[13px] text-muted-foreground/60">
          <span>&copy; {new Date().getFullYear()} Helpin. All rights reserved.</span>
          <div className="flex gap-6">
            <Link href="/privacy" className="hover:text-foreground transition-colors">Privacy</Link>
            <Link href="/terms" className="hover:text-foreground transition-colors">Terms</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
