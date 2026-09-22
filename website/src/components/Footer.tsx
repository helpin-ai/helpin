'use client';

import { usePathname } from 'next/navigation';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';

const SIGNUP_URL = 'https://app.helpin.ai/register';
const DEMO_URL = 'https://cal.com/helpin-ai/30min';

const FOOTER_LINKS = {
  Legal: [
    { label: 'Privacy Policy', href: '/privacy' },
    { label: 'Terms of Service', href: '/terms' },
  ],
};

export function Footer() {
  const pathname = usePathname();
  const isPricing = pathname === '/pricing';

  // The new-site pages and pricing use the shared marketing footer.
  if (pathname?.startsWith('/new') || pathname === '/pricing' || pathname === '/privacy' || pathname === '/terms') return null;

  return (
    <footer className="relative" style={{ backgroundImage: 'image-set(url(/images/footer-bg.webp) type("image/webp"), url(/images/footer-bg.jpg) type("image/jpeg"))', backgroundSize: 'cover', backgroundPosition: 'center bottom' }}>
      {/* Dark overlay */}
      <div className="absolute inset-0 bg-black/60" />

      <div className="relative">
        {/* CTA Section */}
        <div className="mx-auto max-w-7xl px-6 lg:px-8 pt-20 md:pt-32 pb-16 md:pb-24">
          <div className="max-w-3xl mx-auto text-center">
            <h2 className="text-[clamp(2rem,4vw,3rem)] font-bold leading-[1.08] tracking-tight text-white mb-8">
              {isPricing
                ? 'Start with a 14-day Growth trial.'
                : 'The way companies operate is changing, don\'t get left behind.'}
            </h2>
            <p className="text-[17px] text-white/75 leading-relaxed mb-10 max-w-lg mx-auto">
              {isPricing
                ? 'Try every module with AI agents, then choose Starter or Growth when you are ready.'
                : 'Bring your project management, support, sales, and docs into one system — and let agents start moving work forward from day one.'}
            </p>

            <div className="flex flex-col items-center justify-center gap-3 sm:flex-row">
              <Link href={SIGNUP_URL} className="rounded-xl bg-white px-8 py-3.5 text-[15px] font-semibold text-foreground transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-white/10">
                Start free trial <ArrowRight className="inline h-4 w-4 ml-1" />
              </Link>
              <Link href={DEMO_URL} target="_blank" rel="noopener noreferrer" className="rounded-xl border border-white/15 px-8 py-3.5 text-[15px] font-semibold text-white/70 transition-colors hover:bg-white/10 hover:text-white">
                Book a demo
              </Link>
            </div>
            <p className="mt-4 text-sm font-medium text-white/60">No credit card required.</p>
          </div>
        </div>

        {/* Divider */}
        <div className="mx-auto max-w-7xl px-6 lg:px-8">
          <div className="border-t border-white/10" />
        </div>

        {/* Footer links */}
        <div className="mx-auto max-w-7xl px-6 py-12 lg:px-8">
          <div className="flex flex-col sm:flex-row justify-between gap-10">
            <div>
              <Link href="/">
                <HelpinBrand
                  variant="light-on-dark"
                  className="text-white text-[1.5rem]"
                  iconClassName="h-9 w-9"
                />
              </Link>
              <p className="mt-4 text-sm leading-relaxed text-white/70 max-w-xs">
                PM, CRM, support, sales & docs — connected
                by AI agents that do the work.
              </p>
            </div>

            {Object.entries(FOOTER_LINKS).map(([heading, links]) => (
              <div key={heading} className="text-left sm:text-right">
                <h3 className="text-sm font-bold tracking-wide text-white/80">{heading}</h3>
                <ul className="mt-4 space-y-3">
                  {links.map((link) => (
                    <li key={link.href}>
                      <Link
                        href={link.href}
                        className="text-sm text-white/60 hover:text-white transition-colors"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

          <div className="mt-10 border-t border-white/10 pt-6 flex flex-wrap items-center justify-between gap-4 text-[13px] text-white/60">
            <span>&copy; {new Date().getFullYear()} Helpin. All rights reserved.</span>
            <div className="flex gap-6">
              <Link href="/privacy" className="hover:text-white transition-colors">Privacy</Link>
              <Link href="/terms" className="hover:text-white transition-colors">Terms</Link>
            </div>
          </div>
        </div>
      </div>
    </footer>
  );
}
