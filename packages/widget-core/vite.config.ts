import { resolve } from 'path';
import { defineConfig } from 'vite';
import dts from 'vite-plugin-dts';

export default defineConfig({
  resolve: {
    alias: [
      { find: /^@helpin\/shared$/, replacement: resolve(__dirname, '../shared/src/index.ts') },
    ],
  },
  plugins: [dts({ rollupTypes: true })],
  build: {
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      formats: ['es'],
      fileName: 'index',
    },
    rollupOptions: {
      // preact is bundled into the dist so widget-core is self-contained
      // and avoids dual-instance __H errors when used in React host apps
      external: [],
    },
  },
});
