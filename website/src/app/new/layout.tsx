import type { Metadata } from 'next';
import { Instrument_Sans, JetBrains_Mono } from 'next/font/google';
import { ReviewNotesProvider, ReviewToggle } from './_components/ReviewNotes';
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
  description: 'Open-source support, docs, projects, and CRM for SaaS teams.',
  robots: { index: false, follow: false, googleBot: { index: false, follow: false } },
  alternates: { canonical: '/new' },
};

export default function NewHomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <ReviewNotesProvider>
      <div className={`hp3 ${instrumentSans.variable} ${jetbrainsMono.variable}`}>
        {children}
        <ReviewToggle />
      </div>
    </ReviewNotesProvider>
  );
}
