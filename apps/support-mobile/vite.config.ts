import path from 'path'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const host = process.env.TAURI_DEV_HOST

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  // Optional dev-only reverse proxy. iOS App Transport Security blocks
  // plaintext HTTP requests to any host except `localhost`, so when the
  // backend is a plain-HTTP dev server (e.g. http://<ip>:8080) the webview
  // can't reach it directly. Set VITE_API_PROXY_TARGET to that backend and
  // point VITE_API_URL at http://localhost:5176/api — the app then talks to
  // localhost (ATS-exempt) and Vite forwards /api (and the /api/ws
  // websocket) to the real backend server-side, where ATS doesn't apply.
  const proxyTarget = env.VITE_API_PROXY_TARGET

  return {
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
      proxy: proxyTarget
        ? {
            '/api': {
              target: proxyTarget,
              changeOrigin: true,
              ws: true,
            },
          }
        : undefined,
    },
  }
})
