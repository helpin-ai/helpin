/**
 * Builds a clean ESM bundle (helpin.es.js) for npm consumers.
 * Separate from the main Vite build to avoid AMD detection issues.
 */
import { build } from 'vite';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

await build({
  configFile: false,
  resolve: {
    alias: [
      { find: '@helpin-ai/widget-core', replacement: resolve(__dirname, '../../widget-core/src/index.ts') },
      { find: '@helpin-ai/shared', replacement: resolve(__dirname, '../../shared/src/index.ts') },
      { find: /^\.\.\/transport\/https$/, replacement: resolve(__dirname, '../src/transport/https.browser.ts') },
    ],
  },
  build: {
    emptyOutDir: false,
    minify: false,
    lib: {
      entry: resolve(__dirname, '../src/esm-entry.ts'),
      formats: ['es'],
      fileName: () => 'helpin.es.js',
    },
    outDir: resolve(__dirname, '../dist'),
    rollupOptions: {
      external: [],
    },
  },
  logLevel: 'warn',
});

console.log('ESM bundle built: dist/helpin.es.js');
