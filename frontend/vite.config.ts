import path from "path"
import { defineConfig } from 'vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [TanStackRouterVite({ autoCodeSplitting: true }), react(), tailwindcss()],
  server: {
    allowedHosts: ["helpin-dev-fe.tryunhide.com"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@helpin/shared": path.resolve(__dirname, "../packages/shared/src/index.ts"),
      "@helpin/widget-core/styles": path.resolve(__dirname, "../packages/widget-core/src/styles/widget.css"),
      "@helpin/widget-core": path.resolve(__dirname, "../packages/widget-core/dist/index.js"),
    },
  },
})
