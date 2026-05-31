import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

const apiTarget = process.env.SPIDER_API_TARGET || 'http://localhost:8000'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
    outDir: '../cmd/spider/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'shiki': ['shiki'],
          'vue-vendor': ['vue', 'vue-router'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': apiTarget,
      '/sse': apiTarget,
      '/message': apiTarget,
    },
  },
})
