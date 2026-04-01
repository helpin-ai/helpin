import path from "path"
import { defineConfig } from 'vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ mode }) => ({
  plugins: [
    TanStackRouterVite({ autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    mode === 'development' && process.env.VITE_REACT_SCAN === 'true' && {
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
    allowedHosts: ["helpin-dev-fe.tryunhide.com", "dev-azhar.helpin.ai"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@helpin/shared": path.resolve(__dirname, "../packages/shared/src/index.ts"),
      "@helpin/widget-core/styles": path.resolve(__dirname, "../packages/widget-core/src/styles/widget.css"),
      "@helpin/widget-core": path.resolve(__dirname, "../packages/widget-core/dist/index.js"),
    },
  },
}))
