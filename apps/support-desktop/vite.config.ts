import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const host = process.env.TAURI_DEV_HOST
const frontendSrc = path.resolve(__dirname, '../../frontend/src')
const desktopSrc = path.resolve(__dirname, './src')

export default defineConfig({
  clearScreen: false,
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: [
      {
        find: '@/components/pm/task-detail/taskRouteNavigation',
        replacement: path.resolve(__dirname, './src/shims/taskRouteNavigation.ts'),
      },
      {
        find: '@desktop',
        replacement: desktopSrc,
      },
      {
        find: '@',
        replacement: frontendSrc,
      },
      {
        find: '@helpin-ai/support-core',
        replacement: path.resolve(__dirname, '../../packages/support-core/src/index.ts'),
      },
      {
        find: '@helpin-ai/shared',
        replacement: path.resolve(__dirname, '../../packages/shared/src/index.ts'),
      },
      {
        find: '@helpin-ai/widget-core/styles',
        replacement: path.resolve(__dirname, '../../packages/widget-core/src/styles/widget.css'),
      },
      {
        find: '@helpin-ai/widget-core',
        replacement: path.resolve(__dirname, '../../packages/widget-core/src/index.ts'),
      },
    ],
  },
  server: {
    port: 5175,
    strictPort: true,
    host: host || false,
    hmr: host
      ? {
          protocol: 'ws',
          host,
          port: 1421,
        }
      : undefined,
    watch: {
      ignored: ['**/src-tauri/**'],
    },
  },
  envPrefix: ['VITE_', 'TAURI_ENV_*'],
  build: {
    target: process.env.TAURI_ENV_PLATFORM === 'windows' ? 'chrome105' : 'safari13',
    minify: !process.env.TAURI_ENV_DEBUG ? 'esbuild' : false,
    sourcemap: !!process.env.TAURI_ENV_DEBUG,
  },
})
