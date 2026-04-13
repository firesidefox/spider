import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../cmd/spider/web/dist',
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
      '/api': 'http://localhost:9090',
      '/sse': 'http://localhost:9090',
      '/message': 'http://localhost:9090',
    },
  },
})
