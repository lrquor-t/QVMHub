import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true, // 支持 WebSocket（noVNC 需要）
        timeout: 0, // 大文件上传不超时（0=无限制）
        proxyTimeout: 0 // 代理到后端的连接不超时
        // 不 rewrite，后端路由就是 /api 前缀
      }
    }
  }
})
