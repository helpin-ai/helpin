/// <reference types="vitest" />
import path from "path"
import { defineConfig, loadEnv } from 'vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const hmrHost = env.VITE_HMR_HOST?.trim()
  const hmrClientPort = Number(env.VITE_HMR_CLIENT_PORT) || undefined

  return {
  plugins: [
    TanStackRouterVite({ autoCodeSplitting: true }),
    react({
      babel: {
        plugins: ['babel-plugin-react-compiler'],
      },
    }),
    tailwindcss(),
    mode === 'development' && env.VITE_REACT_SCAN === 'true' && {
      name: 'react-scan',
      transformIndexHtml(html: string) {
        return html.replace(
          '<head>',
          '<head><script src="https://unpkg.com/react-scan/dist/auto.global.js" crossorigin="anonymous"></script>',
        );
      },
    },
  ].filter(Boolean),
  server: {
    headers: {
      'Cache-Control': 'no-store',
    },
    allowedHosts: ["helpin-dev-fe.tryunhide.com", "dev-azhar.helpin.ai", "azhar.dev.helpin.ai"],
    ...(hmrHost ? {
      hmr: { host: hmrHost, protocol: "wss", clientPort: hmrClientPort ?? 443 },
    } : {}),
  },
  resolve: {
    dedupe: ['react', 'react-dom', '@tanstack/react-query', 'zustand'],
    alias: {
      "@": path.resolve(__dirname, "./src"),
      // The published package declares lib/index.js as its main entry but only
      // ships the ESM build. Resolve that shipped entry explicitly for Vite.
      "@helpin-ai/react": path.resolve(__dirname, "./node_modules/@helpin-ai/react/lib/index.es.js"),
      "@helpin-ai/shared": path.resolve(__dirname, "../packages/shared/src/index.ts"),
      "@helpin-ai/support-core": path.resolve(__dirname, "../packages/support-core/src/index.ts"),
      "@helpin-ai/widget-core/styles": path.resolve(__dirname, "../packages/widget-core/src/styles/widget.css"),
      "@helpin-ai/widget-core": path.resolve(__dirname, "../packages/widget-core/dist/index.js"),
    },
  },
  // Vitest config — colocated with vite to inherit aliases + plugins.
  // Per-file `// @vitest-environment jsdom` directives keep working.
  test: {
    exclude: [
      '**/node_modules/**',
      '**/dist/**',
      '**/e2e/**',
      '**/.idea/**',
      '**/.git/**',
      '**/.cache/**',
    ],
  },
}})
