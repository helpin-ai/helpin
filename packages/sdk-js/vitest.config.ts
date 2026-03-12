import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./test/setup.ts'],
    include: ['test/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    exclude: [
      '**/node_modules/**',
      '**/test/e2e/**',
      '**/node_modules/.pnpm/**'
    ],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.ts'],
    },
  },
  resolve: {
    alias: [
      { find: '@', replacement: path.resolve(__dirname, './src') },
      { find: /^@helpin\/widget-core\/styles/, replacement: path.resolve(__dirname, './test/__mocks__/widget-styles.ts') },
      { find: /^@helpin\/widget-core$/, replacement: path.resolve(__dirname, './test/__mocks__/widget-core.ts') },
      { find: /^@helpin\/shared$/, replacement: path.resolve(__dirname, '../shared/dist/index.js') },
    ],
  },
});
