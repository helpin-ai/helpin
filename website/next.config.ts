import type { NextConfig } from 'next';
import { PHASE_DEVELOPMENT_SERVER } from 'next/constants';

export default function nextConfig(phase: string): NextConfig {
  const development = phase === PHASE_DEVELOPMENT_SERVER;

  return {
    output: development ? undefined : 'export',
    // Static export always builds through .next, even with a custom distDir
    // (which Next treats as the export destination). Isolate the dev cache.
    distDir: development ? '.next-dev' : (process.env.NEXT_DIST_DIR ?? '.next'),
    images: {
      formats: ['image/avif', 'image/webp'],
    },
  };
}
