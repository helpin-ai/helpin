import { defineConfig } from 'vite';
import { resolve } from 'path';
import { readFileSync } from 'fs';
import { globSync } from 'glob';
import dts from 'vite-plugin-dts';

/**
 * Vite plugin that injects the hashed SDK filename into the loader (lib.js).
 * Replaces __SDK_FILENAME__ with the actual content-hashed filename after build.
 */
function injectSDKFilename() {
  return {
    name: 'inject-sdk-filename',
    writeBundle(options: any, bundle: Record<string, any>) {
      // Find the hashed SDK bundle filename
      const sdkChunk = Object.values(bundle).find(
        (chunk: any) => chunk.type === 'chunk' && chunk.facadeModuleId?.endsWith('index.ts')
      );
      if (!sdkChunk) return;

      const sdkFilename = (sdkChunk as any).fileName;
      const outDir = options.dir || 'dist';
      const loaderPath = resolve(outDir, 'lib.js');

      try {
        let loaderCode = readFileSync(loaderPath, 'utf-8');
        loaderCode = loaderCode.replace(/__SDK_FILENAME__/g, sdkFilename);
        require('fs').writeFileSync(loaderPath, loaderCode);
      } catch {
        // loader might not exist yet in watch mode
      }
    },
  };
}

export default defineConfig(({ command }) => {
  const isBuild = command === 'build';

  return {
    resolve: {
      alias: [
        { find: /^@helpin-ai\/widget-core\/styles/, replacement: resolve(__dirname, '../widget-core/src/styles/widget.css') },
        { find: /^@helpin-ai\/widget-core$/, replacement: resolve(__dirname, '../widget-core/src/index.ts') },
        { find: /^@helpin-ai\/shared$/, replacement: resolve(__dirname, '../shared/src/index.ts') },
        { find: /^\.\.\/transport\/https$/, replacement: resolve(__dirname, 'src/transport/https.browser.ts') },
      ],
    },
    build: {
      cssCodeSplit: false,
      rollupOptions: {
        input: {
          loader: resolve(__dirname, 'src/loader.ts'),
          index: resolve(__dirname, 'src/index.ts'),
        },
        external: [],
        output: {
          entryFileNames: (chunkInfo) => {
            if (chunkInfo.name === 'loader') return 'lib.js';
            return 'helpin.[hash].js';
          },
          chunkFileNames: 'chunks/[name].[hash].js',
          assetFileNames: (assetInfo) => {
            if (assetInfo.names?.[0]?.endsWith('.css')) return 'helpin.[hash].css';
            return 'assets/[name].[hash][extname]';
          },
          format: 'es',
        },
      },
    },
    plugins: [
      isBuild &&
        dts({
          insertTypesEntry: true,
          include: ['src/**/*.ts'],
          exclude: ['test', 'node_modules', 'src/loader.ts'],
          outDir: 'dist',
        }),
      isBuild && injectSDKFilename(),
    ].filter(Boolean),
    server: {
      open: '/examples/index.html',
      watch: {
        usePolling: true,
        ignored: ['!**/dist/**'],
      },
    },
    optimizeDeps: {
      exclude: ['@helpin-ai/sdk-js'],
    },
  };
});
