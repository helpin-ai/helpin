import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// Isolated design entry: no auth, API, router generation, or production route edits.
export default defineConfig({
  root: path.resolve(__dirname, '../..'),
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(__dirname, '../../src') } },
  server: { host: '0.0.0.0', port: 5187, strictPort: true },
  build: {
    outDir: 'design-preview/signals/dist',
    rollupOptions: { input: path.resolve(__dirname, 'index.html') },
  },
});
