'use client';

import { useState, useEffect } from 'react';
import { usePathname } from 'next/navigation';
import Link from 'next/link';
import { Menu, X } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';

const SIGNUP_URL = 'https://app.helpin.ai/register';

const NAV_LINKS: { label: string; href: string }[] = [
  { label: 'Agents', href: '/#agents' },
  { label: 'Pricing', href: '/pricing' },
];

export function Navbar() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);

  const pathname = usePathname();

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 16);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  // The new-site pages and pricing use the shared marketing navigation.
  if (pathname?.startsWith('/new') || pathname === '/pricing') return null;

  return (
    <header
      className={`sticky top-0 z-50 transition-all duration-300 ${
        scrolled
          ? 'border-b border-border/40 bg-background/80 backdrop-blur-xl shadow-sm'
          : 'bg-transparent'
      }`}
    >
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4 lg:px-8">
        <Link href="/">
          <HelpinBrand />
        </Link>

        <div className="hidden md:flex items-center gap-8">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="text-[15px] font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              {link.label}
            </Link>
          ))}
        </div>

        <div className="hidden md:flex items-center gap-4">
          <Link
            href="https://app.helpin.ai"
            className="text-[15px] font-medium text-muted-foreground hover:text-foreground transition-colors"
          >
            Log in
          </Link>
          <Link
            href={SIGNUP_URL}
            className="rounded-xl bg-foreground px-5 py-2.5 text-[15px] font-semibold text-background transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-foreground/10"
          >
            Start free trial
          </Link>
        </div>

        <button
          type="button"
          className="md:hidden p-1 text-muted-foreground"
          onClick={() => setMobileOpen(!mobileOpen)}
        >
          {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </nav>

      {mobileOpen && (
        <div className="md:hidden border-t border-border/40 bg-background/95 backdrop-blur-xl px-6 py-4 space-y-3">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="block text-sm text-muted-foreground hover:text-foreground"
              onClick={() => setMobileOpen(false)}
            >
              {link.label}
            </Link>
          ))}
          <div className="pt-2 flex flex-col gap-2">
            <Link
              href="https://app.helpin.ai"
              className="text-sm text-muted-foreground hover:text-foreground"
              onClick={() => setMobileOpen(false)}
            >
              Log in
            </Link>
            <Link
              href={SIGNUP_URL}
              className="rounded-xl bg-foreground px-4 py-2.5 text-center text-sm font-semibold text-background"
              onClick={() => setMobileOpen(false)}
            >
              Start free trial
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}
