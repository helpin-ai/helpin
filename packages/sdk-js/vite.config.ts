import { defineConfig } from 'vite';
import { resolve } from 'path';
import { readFileSync } from 'fs';
import { globSync } from 'glob';
import dts from 'vite-plugin-dts';

/**
 * Vite plugin that:
 * 1. Injects the hashed SDK filename into the loader (lib.js) for CDN cache busting
 * 2. Copies the hashed SDK bundle to helpin.es.js for npm consumers (stable filename)
 */
function postBuildSDK() {
  return {
    name: 'post-build-sdk',
    writeBundle(options: any, bundle: Record<string, any>) {
      const fs = require('fs');
      // Find the hashed SDK bundle filename
      const sdkChunk = Object.values(bundle).find(
        (chunk: any) => chunk.type === 'chunk' && chunk.facadeModuleId?.endsWith('index.ts')
      );
      if (!sdkChunk) return;

      const sdkFilename = (sdkChunk as any).fileName;
      const outDir = options.dir || 'dist';

      // 1. Inject hashed filename into loader for CDN
      const loaderPath = resolve(outDir, 'lib.js');
      try {
        let loaderCode = readFileSync(loaderPath, 'utf-8');
        loaderCode = loaderCode.replace(/__SDK_FILENAME__/g, sdkFilename);
        fs.writeFileSync(loaderPath, loaderCode);
      } catch {
        // loader might not exist yet in watch mode
      }

      // 2. Copy hashed bundle to stable helpin.es.js for npm imports
      try {
        const hashedPath = resolve(outDir, sdkFilename);
        const stablePath = resolve(outDir, 'helpin.es.js');
        if (hashedPath !== stablePath) {
          fs.copyFileSync(hashedPath, stablePath);
        }
      } catch {
        // ignore in watch mode
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
      isBuild && postBuildSDK(),
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
