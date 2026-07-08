import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const host = process.env.TAURI_DEV_HOST

export default defineConfig({
  clearScreen: false,
  plugins: [react(), tailwindcss()],
  resolve: {
    dedupe: ['react', 'react-dom', '@tanstack/react-query', 'zustand'],
    alias: [
      { find: '@mobile', replacement: path.resolve(__dirname, './src') },
      {
        find: '@helpin-ai/support-core',
        replacement: path.resolve(__dirname, '../../packages/support-core/src/index.ts'),
      },
      {
        find: '@helpin-ai/shared',
        replacement: path.resolve(__dirname, '../../packages/shared/src/index.ts'),
      },
      {
        find: '@helpin/plugin-push',
        replacement: path.resolve(
          __dirname,
          './src-tauri/tauri-plugin-helpin-push/guest-js/index.ts',
        ),
      },
    ],
  },
  server: {
    port: 5176,
    strictPort: true,
    host: host || '0.0.0.0',
    hmr: host ? { protocol: 'ws', host, port: 5177 } : undefined,
    watch: { ignored: ['**/src-tauri/**'] },
  },
})
