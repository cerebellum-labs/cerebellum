import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    // Matches "paths" in tsconfig.json: @/components/ui/button → src/components/ui/button.
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    // In development the page and the API share this origin in the browser:
    // /api goes to the server (make db run in packages/server), the WebSocket
    // included. The server still sees the page's Origin, and `make run`
    // allows http://localhost:5173 only, so the port must not move.
    strictPort: true,
    proxy: {
      // changeOrigin sends the target's own Host, so a deployed API behind
      // Cloudflare answers too.
      '/api': { target: process.env.API_PROXY_TARGET ?? 'http://localhost:8080', ws: true, changeOrigin: true },
    },
  },
})
