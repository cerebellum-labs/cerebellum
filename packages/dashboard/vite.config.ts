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
})
