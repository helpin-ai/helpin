import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

export default defineConfig({
  plugins: [preact()],
  build: {
    lib: {
      entry: 'src/main.tsx',
      name: 'HelpinWidget',
      formats: ['iife'],
      fileName: () => 'pixel.js',
    },
    rollupOptions: {
      output: {
        inlineDynamicImports: true,
      },
    },
  },
});
