import type { Metadata } from 'next';
import Script from 'next/script';
import { Plus_Jakarta_Sans, Instrument_Serif } from 'next/font/google';
import { Navbar } from '@/components/Navbar';
import { Footer } from '@/components/Footer';
import './globals.css';

const jakarta = Plus_Jakarta_Sans({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
});

const instrumentSerif = Instrument_Serif({
  weight: '400',
  style: ['normal', 'italic'],
  subsets: ['latin'],
  variable: '--font-display',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Helpin — The AI Operating System for Modern Work',
  description:
    'Helpin brings project management, support, sales, and docs into one connected system. AI agents plan, build, triage, and follow up — so your team moves faster without the chaos.',
  metadataBase: new URL('https://helpin.ai'),
  openGraph: {
    title: 'Helpin — The AI Operating System for Modern Work',
    description:
      'One connected system for PM, support, sales, and docs. AI agents that actually do the work.',
    url: 'https://helpin.ai',
    siteName: 'Helpin',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Helpin — The AI Operating System for Modern Work',
    description:
      'One connected system for PM, support, sales, and docs. AI agents that actually do the work.',
  },
  robots: {
    index: true,
    follow: true,
  },
  icons: {
    icon: '/favicon.ico',
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={`${jakarta.variable} ${instrumentSerif.variable}`}>
      <body className="min-h-screen bg-background font-sans text-foreground antialiased">
        <Navbar />
        <main>{children}</main>
        <Footer />

        {/* Usermaven */}
        <Script
          id="um-tracker"
          strategy="afterInteractive"
          data-tracking-host="https://events.usermaven.com"
          data-key="UMpgKYZLxR"
          data-autocapture="true"
          data-form-tracking="all"
          src="https://t.usermaven.com/lib.js"
        />
        <Script id="um-init" strategy="afterInteractive">{`
          window.usermaven = window.usermaven || function(){ (window.usermavenQ = window.usermavenQ || []).push(arguments); };
        `}</Script>

        {/* Customer.io */}
        <Script id="cio-init" strategy="afterInteractive">{`
          var _cio = _cio || [];
          (function(){
            var a,b,c;a=function(f){return function(){_cio.push([f].concat(Array.prototype.slice.call(arguments,0)))}};b=["load","identify","sidentify","track","page"];for(c=0;c<b.length;c++){_cio[b[c]]=a(b[c])};
          })();
        `}</Script>
        <Script
          id="cio-tracker"
          strategy="afterInteractive"
          data-site-id="a3fced22111b6be05726"
          data-use-array-params="true"
          data-auto-track-page="true"
          src="https://assets.customer.io/assets/track.js"
        />
      </body>
    </html>
  );
}
