import type { Metadata, Viewport } from 'next';
import Script from 'next/script';
import { Plus_Jakarta_Sans, Instrument_Serif } from 'next/font/google';
import { Navbar } from '@/components/Navbar';
import { Footer } from '@/components/Footer';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
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
  ...createPageMetadata(PAGE_SEO.home),
  applicationName: 'Helpin',
  metadataBase: new URL(SITE_URL),
  icons: {
    icon: [
      { url: '/favicon.ico?v=20260804-3', sizes: 'any' },
      { url: '/favicon.svg?v=20260804-3', type: 'image/svg+xml', media: '(prefers-color-scheme: light)' },
      { url: '/favicon-dark.svg?v=20260804-3', type: 'image/svg+xml', media: '(prefers-color-scheme: dark)' },
      { url: '/favicon-32x32.png?v=20260804-3', type: 'image/png', sizes: '32x32', media: '(prefers-color-scheme: light)' },
      { url: '/favicon-32x32-dark.png?v=20260804-3', type: 'image/png', sizes: '32x32', media: '(prefers-color-scheme: dark)' },
      { url: '/favicon-16x16.png?v=20260804-3', type: 'image/png', sizes: '16x16', media: '(prefers-color-scheme: light)' },
      { url: '/favicon-16x16-dark.png?v=20260804-3', type: 'image/png', sizes: '16x16', media: '(prefers-color-scheme: dark)' },
    ],
    apple: '/apple-touch-icon.png?v=20260804',
  },
  manifest: '/site.webmanifest?v=20260804',
};

export const viewport: Viewport = {
  themeColor: '#1E1C1A',
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
        <Script id="cio-tracker" strategy="afterInteractive">{`
          !function(){
            var i,o,a=window.analytics=window.analytics||[];
            if(!a.initialize)if(a.invoked)window.console&&console.error&&console.error("Customer.io snippet included twice.");
            else{
              a.invoked=!0;
              a.methods=["trackSubmit","trackClick","trackLink","trackForm","pageview","identify","reset","group","track","ready","alias","debug","page","once","off","on","addSourceMiddleware","addIntegrationMiddleware","setAnonymousId","addDestinationMiddleware"];
              a.factory=function(i){return function(){var o=Array.prototype.slice.call(arguments);return o.unshift(i),a.push(o),a}};
              for(i=0;i<a.methods.length;i++)o=a.methods[i],a[o]=a.factory(o);
              a.load=function(i,o){
                var n,t=document.createElement("script");
                t.type="text/javascript";
                t.async=!0;
                t.src="https://cdp.customer.io/v1/analytics-js/snippet/"+i+"/analytics.min.js";
                n=document.getElementsByTagName("script")[0];
                n.parentNode.insertBefore(t,n);
                a._writeKey=i;
                a._loadOptions=o;
              };
              a.SNIPPET_VERSION="4.15.3";
              a.load("a3fced22111b6be05726");
              a.page();
            }
          }();
        `}</Script>
      </body>
    </html>
  );
}
