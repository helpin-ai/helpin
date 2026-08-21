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
          find: '@helpin-ai/widget-core/emoji-loader',
          replacement: path.resolve(__dirname, '../../packages/widget-core/src/components/emoji-loader.ts'),
        },
        {
          find: '@helpin-ai/widget-core/emoji-catalog',
          replacement: path.resolve(__dirname, '../../packages/widget-core/src/components/emoji-catalog.ts'),
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
        // Read-in-place reuse of the web support "rulebook" (pure logic only).
        // Explicit per-path aliases — deliberately NOT a blanket `@ → frontend/src`,
        // so web UI can never be accidentally imported into the mobile bundle. The
        // mobile inbox derives every view's server query from the web's own
        // `buildConversationListRequestFilters`, guaranteeing identical results.
        {
          find: '@/lib/supportInboxFilters',
          replacement: path.resolve(__dirname, '../../frontend/src/lib/supportInboxFilters.ts'),
        },
        {
          find: '@/lib/supportInboxRouting',
          replacement: path.resolve(__dirname, '../../frontend/src/lib/supportInboxRouting.ts'),
        },
        {
          find: '@/lib/favicon',
          replacement: path.resolve(__dirname, '../../frontend/src/lib/favicon.ts'),
        },
        {
          find: '@/lib/teamMemberAvatar',
          replacement: path.resolve(__dirname, '../../frontend/src/lib/teamMemberAvatar.ts'),
        },
        {
          find: '@/lib/pmTypes',
          replacement: path.resolve(__dirname, '../../frontend/src/lib/pmTypes.ts'),
        },
        {
          find: '@/stores/supportInboxStore',
          replacement: path.resolve(__dirname, '../../frontend/src/stores/supportInboxStore.ts'),
        },
        {
          find: '@/components/support/helpers',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/helpers.ts'),
        },
        {
          find: '@/components/support/conversationRowVisual',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/conversationRowVisual.ts'),
        },
        {
          find: '@/components/support/supportSystemEvent',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/supportSystemEvent.ts'),
        },
        {
          find: '@/components/support/shortcutFiltering',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/shortcutFiltering.ts'),
        },
        {
          find: '@/components/support/shortcutVariables',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/shortcutVariables.ts'),
        },
        {
          find: '@/components/support/shortcutCategories',
          replacement: path.resolve(__dirname, '../../frontend/src/components/support/shortcutCategories.ts'),
        },
        {
          find: '@/components/agents/dock/starterSuggestions',
          replacement: path.resolve(__dirname, '../../frontend/src/components/agents/dock/starterSuggestions.ts'),
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
