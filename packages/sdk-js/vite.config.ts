import { defineConfig } from 'vite';
import { resolve } from 'path';
import dts from 'vite-plugin-dts';

export default defineConfig(({ command, mode }) => {
  // Determine if we are in build mode
  const isBuild = command === 'build';

  return {
    resolve: {
      alias: [
        { find: /^@helpin\/widget-core\/styles/, replacement: resolve(__dirname, '../widget-core/src/styles/widget.css') },
        { find: /^@helpin\/widget-core$/, replacement: resolve(__dirname, '../widget-core/dist/index.js') },
        { find: /^@helpin\/shared$/, replacement: resolve(__dirname, '../shared/dist/index.js') },
      ],
    },
    build: {
      lib: {
        entry: resolve(__dirname, 'src/index.ts'),
        name: 'Helpin',
        formats: ['es', 'cjs', 'umd'],
        fileName: (format) => {
          if (format === 'umd') {
            return 'lib.js';
          }
          return `helpin.${format}.js`;
        },
      },
      cssCodeSplit: false,
      rollupOptions: {
        external: [], // Everything bundled inline (including widget-core + preact)
        output: {
          globals: {
            module: 'module',
          },
        },
      },
    },
    plugins: [
      // Conditionally include the dts plugin only during build
      isBuild &&
        dts({
          insertTypesEntry: true,
          include: ['src/**/*.ts'],
          exclude: ['test', 'node_modules'],
          outDir: 'dist',
        }),
    ].filter(Boolean),
    server: {
      open: '/examples/index.html',
      watch: {
        usePolling: true,
        ignored: ['!**/dist/**'],
      },
    },
    optimizeDeps: {
      exclude: ['@helpin/sdk-js'],
    },
  };
});
