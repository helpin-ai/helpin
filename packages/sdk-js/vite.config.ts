import { defineConfig } from 'vite';
import { resolve } from 'path';
import dts from 'vite-plugin-dts';

export default defineConfig(({ command, mode }) => {
  // Determine if we are in build mode
  const isBuild = command === 'build';

  return {
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
      rollupOptions: {
        external: [], // Add external dependencies here if needed
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
          outDir: 'dist', // Changed from outputDir to outDir
        }),
      // Add other plugins here if needed
    ].filter(Boolean), // Filter out any falsey values (like 'false') to avoid Vite warnings
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
