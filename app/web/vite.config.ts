import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { homeUi } from '@mqtt-home/ui/vite'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  // homeUi: dark mode + Wall Display detection before first paint
  plugins: [react(), tailwindcss(), homeUi()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
