import { Instrument_Sans, JetBrains_Mono } from 'next/font/google';
import { ScrollReveal } from './ScrollReveal';
import '../new.css';
import './preview-scrolling.css';
import './product-rhythm.css';

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

export function MarketingShell({ children }: { children: React.ReactNode }) {
  return (
    <div className={`hp3 ${instrumentSans.variable} ${jetbrainsMono.variable}`}>
      {children}
      <ScrollReveal />
    </div>
  );
}
