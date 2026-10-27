import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

// Standalone Community UI preview. No API, authentication, or production routes.
export default defineConfig({
  root: __dirname,
  plugins: [react(), tailwindcss()],
  resolve: { alias: {
    '@': path.resolve(__dirname, '../../src'),
    '@edition': path.resolve(__dirname, '../../src/edition/community'),
    '@helpin-ai/shared': path.resolve(__dirname, '../../../packages/shared/src/index.ts'),
    '@helpin-ai/support-core': path.resolve(__dirname, '../../../packages/support-core/src/index.ts'),
    '@helpin-ai/widget-core/styles': path.resolve(__dirname, '../../../packages/widget-core/src/styles/widget.css'),
    '@helpin-ai/widget-core': path.resolve(__dirname, '../../../packages/widget-core/dist/index.js'),
  }, dedupe: ['react', 'react-dom', '@tanstack/react-query', 'zustand'] },
  build: { outDir: '/tmp/helpin-moderation-preview', emptyOutDir: true },
});
