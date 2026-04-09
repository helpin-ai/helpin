import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  test: {
    globals: true,
    environment: 'jsdom',
    include: ['test/**/*.{test,spec}.{ts,tsx}'],
    exclude: ['**/node_modules/**'],
  },
  resolve: {
    alias: [
      {
        find: '@helpin-ai/sdk-js',
        replacement: path.resolve(__dirname, './test/__mocks__/sdk-js.ts'),
      },
    ],
  },
});
