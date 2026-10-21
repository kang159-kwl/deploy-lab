import { defineConfig } from 'vite'

export default defineConfig({
  server: {
    port: 5175,
    // 开发时前端在 5173，后端在 8080，用代理打通，避免跨域。
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
