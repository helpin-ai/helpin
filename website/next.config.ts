import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'export',
  // Let a production build run beside `next dev` without clobbering its .next cache.
  distDir: process.env.NEXT_DIST_DIR ?? '.next',
  images: {
    formats: ['image/avif', 'image/webp'],
  },
};

export default nextConfig;
