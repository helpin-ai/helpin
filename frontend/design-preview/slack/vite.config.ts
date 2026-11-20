import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
export default defineConfig({
  root: path.resolve(__dirname, '../..'),
  build: { rollupOptions: { input: path.resolve(__dirname, "index.html") } },
  plugins: [react(), tailwindcss()],
  resolve: { alias: {
    '@': path.resolve(__dirname, '../../src'),
    '@helpin-ai/support-core': path.resolve(__dirname, '../../../packages/support-core/src/index.ts'),
    '@helpin-ai/shared': path.resolve(__dirname, '../../../packages/shared/src/index.ts'),
    '@helpin-ai/widget-core/styles': path.resolve(__dirname, '../../../packages/widget-core/src/styles/widget.css'),
    '@helpin-ai/widget-core': path.resolve(__dirname, '../../../packages/widget-core/dist/index.js'),
    '@edition': path.resolve(__dirname, '../../src/edition/community'),
    '@helpin-ai/react': path.resolve(__dirname, '../../node_modules/@helpin-ai/react/lib/index.es.js'),
  } },
  optimizeDeps: { entries: ['design-preview/slack/index.html'] },
  server: { host: '127.0.0.1', port: 5198, strictPort: true },
});
