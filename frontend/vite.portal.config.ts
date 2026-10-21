import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The portal-only build that help centers serve at /requests. Assets use
// relative URLs; the help center server adds a <base> for the mount path.
export default defineConfig({
  base: './',
  // The app's public/ folder (icons, sounds, and so on) isn't used by the portal.
  publicDir: false,
  plugins: [
    react({ babel: { plugins: ['babel-plugin-react-compiler'] } }),
    tailwindcss(),
  ],
  resolve: {
    dedupe: ['react', 'react-dom'],
    alias: {
      '@edition': path.resolve(__dirname, './src/edition/community'),
      '@': path.resolve(__dirname, './src'),
      '@helpin-ai/shared': path.resolve(__dirname, '../packages/shared/src/index.ts'),
      '@helpin-ai/support-core': path.resolve(__dirname, '../packages/support-core/src/index.ts'),
    },
  },
  build: {
    outDir: 'dist-portal',
    emptyOutDir: true,
    rollupOptions: { input: path.resolve(__dirname, 'portal.html') },
  },
})
