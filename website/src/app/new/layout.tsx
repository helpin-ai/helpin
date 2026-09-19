import type { Metadata } from 'next';
import { Instrument_Sans, JetBrains_Mono } from 'next/font/google';
import { ScrollReveal } from './_components/ScrollReveal';
import './new.css';

const instrumentSans = Instrument_Sans({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  variable: '--font-hp3-sans',
  display: 'swap',
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  weight: ['400', '500'],
  variable: '--font-hp3-mono',
  display: 'swap',
});

// Preview route. Not indexed until the direction is approved and it replaces /.
export const metadata: Metadata = {
  title: 'Helpin — homepage preview',
  description: 'Open-source customer support, product work, and CRM for teams and agents. Answer customers, plan tasks, and prepare follow-ups in one workspace.',
  robots: { index: false, follow: false, googleBot: { index: false, follow: false } },
  alternates: { canonical: '/new' },
};

export default function NewHomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className={`hp3 ${instrumentSans.variable} ${jetbrainsMono.variable}`}>
      {children}
      <ScrollReveal />
    </div>
  );
}
