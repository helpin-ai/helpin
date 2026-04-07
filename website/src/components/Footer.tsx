'use client';

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';

const FOOTER_LINKS = {
  Legal: [
    { label: 'Privacy Policy', href: '/privacy' },
    { label: 'Terms of Service', href: '/terms' },
  ],
};

export function Footer() {
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<'idle' | 'submitting' | 'success'>('idle');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !email.includes('@')) return;
    setStatus('submitting');
    try {
      const w = window as unknown as {
        usermaven?: (cmd: string, ...args: unknown[]) => void;
        _cio?: { identify: (obj: Record<string, unknown>) => void; track: (event: string, obj?: Record<string, unknown>) => void };
      };
      if (w.usermaven) {
        w.usermaven('lead', { email });
        w.usermaven('track', 'early_access_signup', { form_id: 'footer-cta', email });
      }
      if (w._cio) {
        w._cio.identify({ id: email, email, created_at: Math.floor(Date.now() / 1000) });
        w._cio.track('early_access_signup', { form_id: 'footer-cta' });
      }
      setStatus('success');
      setEmail('');
    } catch {
      setStatus('idle');
    }
  };

  return (
    <footer className="relative" style={{ backgroundImage: 'image-set(url(/images/footer-bg.webp) type("image/webp"), url(/images/footer-bg.jpg) type("image/jpeg"))', backgroundSize: 'cover', backgroundPosition: 'center bottom' }}>
      {/* Dark overlay */}
      <div className="absolute inset-0 bg-black/60" />

      <div className="relative">
        {/* CTA Section */}
        <div className="mx-auto max-w-7xl px-6 lg:px-8 pt-20 md:pt-32 pb-16 md:pb-24">
          <div className="max-w-3xl mx-auto text-center">
            <h2 className="text-[clamp(2rem,4vw,3rem)] font-bold leading-[1.08] tracking-tight text-white mb-8">
              The way companies operate is changing, don't get left behind.
            </h2>
            <p className="text-[17px] text-white/50 leading-relaxed mb-10 max-w-lg mx-auto">
              Bring your project management, support, sales, and docs into one system — and let agents start moving work forward from day one.
            </p>

            {/* Email form */}
            {status === 'success' ? (
              <p className="text-[15px] font-medium text-white/70">
                You're on the list. We'll be in touch soon.
              </p>
            ) : (
              <div className="w-full max-w-lg mx-auto">
                <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-3 w-full rounded-[14px] p-2" style={{ background: 'oklch(1 0 0 / 0.08)', border: '1px solid oklch(1 0 0 / 0.1)' }}>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="Enter your work email"
                    required
                    className="flex-1 px-4 py-3 rounded-lg text-[15px] outline-none"
                    style={{ color: 'white', background: 'oklch(1 0 0 / 0.05)', border: '1px solid oklch(1 0 0 / 0.08)' }}
                  />
                  <button
                    type="submit"
                    disabled={status === 'submitting'}
                    className="btn-primary whitespace-nowrap justify-center w-full sm:w-auto"
                    style={{ background: 'white', color: 'var(--color-foreground)' }}
                  >
                    {status === 'submitting' ? 'Submitting...' : 'Get early access'}
                    {status === 'idle' && <ArrowRight className="h-4 w-4" />}
                  </button>
                </form>
              </div>
            )}
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
                <img src="/logos/helpin-light-mode-logo.svg" alt="Helpin" className="h-9 brightness-0 invert" />
              </Link>
              <p className="mt-4 text-sm leading-relaxed text-white/50 max-w-xs">
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
                        className="text-sm text-white/40 hover:text-white transition-colors"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

          <div className="mt-10 border-t border-white/10 pt-6 flex flex-wrap items-center justify-between gap-4 text-[13px] text-white/30">
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
