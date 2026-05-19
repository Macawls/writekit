import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

const version = process.env.APP_VERSION || 'dev'
const repo = process.env.APP_REPO || 'Macawls/writekit'

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(version),
    __APP_REPO__: JSON.stringify(repo),
  },
  base: '/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: process.env.VITE_BACKEND_URL || 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
    },
  },
})
