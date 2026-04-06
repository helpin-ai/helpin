import type { Metadata } from 'next';
import HelpinClientProvider from '@/components/HelpinProvider';

export const metadata: Metadata = {
  title: 'Helpin Next.js Example',
  description: 'Example app demonstrating @helpin-ai/nextjs SDK integration',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <HelpinClientProvider>{children}</HelpinClientProvider>
      </body>
    </html>
  );
}
