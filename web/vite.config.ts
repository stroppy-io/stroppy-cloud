import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// Port 5173 is the origin registered for the IAM "cloud web (dev)" client.
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
      routesDirectory: 'src/routes',
      generatedRouteTree: 'src/routeTree.gen.ts',
    }),
    react(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@app': path.resolve(__dirname, './src/app'),
      '@components': path.resolve(__dirname, './src/components'),
      '@hooks': path.resolve(__dirname, './src/hooks'),
      '@api': path.resolve(__dirname, './src/api'),
      '@lib': path.resolve(__dirname, './src/lib'),
      '@helpers': path.resolve(__dirname, './src/helpers'),
      '@routes': path.resolve(__dirname, './src/routes'),
    },
  },
  preview: {
    port: 4173,
    proxy: {
      '/api': { target: 'http://localhost:18347', changeOrigin: true, ws: true },
      '/v1': { target: 'http://localhost:18347', changeOrigin: true },
      '/grafana': { target: 'http://localhost:18347', changeOrigin: true },
    },
  },
  server: {
    port: 5173,
    proxy: {
      // stroppy-server API + WS; prod serves the SPA from the same origin.
      '/api': { target: 'http://localhost:18347', changeOrigin: true, ws: true },
      // IAM, reverse-proxied by stroppy-server so auth stays same-origin.
      '/v1': { target: 'http://localhost:18347', changeOrigin: true },
      // embedded Grafana relay.
      '/grafana': { target: 'http://localhost:18347', changeOrigin: true },
    },
  },
})
