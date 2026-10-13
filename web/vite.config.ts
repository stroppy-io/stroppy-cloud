import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type ProxyOptions } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// The stand (make dev) answers with absolute URLs of its own origin: the
// Kratos base in /api/v1/public/config, every flow's `ui.action`, logout
// links, redirects. Served from here, those would leave this origin (and its
// cookies) behind. The proxy puts this origin in their place — in the body of
// Kratos and public-config answers and in any `Location` — so the app runs
// unchanged, exactly as it does same-origin in production.
const STAND = 'http://localhost:18347'

function stand(opts: { ws?: boolean } = {}): ProxyOptions {
  return {
    target: STAND,
    changeOrigin: true,
    ws: opts.ws,
    selfHandleResponse: true,
    configure: (proxy) => {
      // Plain bodies, so they can be rewritten.
      proxy.on('proxyReq', (proxyReq) => proxyReq.setHeader('accept-encoding', 'identity'))
      proxy.on('proxyRes', (proxyRes, req, res) => {
        const origin = `http://${req.headers.host}`
        const headers = { ...proxyRes.headers }
        if (typeof headers.location === 'string') {
          headers.location = headers.location.replace(STAND, origin)
        }
        const url = req.url ?? ''
        const rewrite =
          (url.startsWith('/v1') || url.startsWith('/api/v1/public/config')) &&
          /json|text|html/.test(String(headers['content-type'] ?? ''))
        if (!rewrite) {
          res.writeHead(proxyRes.statusCode ?? 502, headers)
          proxyRes.pipe(res)
          return
        }
        const chunks: Buffer[] = []
        proxyRes.on('data', (c: Buffer) => chunks.push(c))
        proxyRes.on('end', () => {
          const body = Buffer.concat(chunks).toString('utf8').split(STAND).join(origin)
          delete headers['content-length']
          res.writeHead(proxyRes.statusCode ?? 502, headers)
          res.end(body)
        })
      })
    },
  }
}

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
      '/api': stand({ ws: true }),
      '/v1': stand(),
      '/grafana': stand(),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // stroppy-server API + WS; prod serves the SPA from the same origin.
      '/api': stand({ ws: true }),
      // Kratos, reverse-proxied by stroppy-server so auth stays same-origin.
      '/v1': stand(),
      // embedded Grafana relay.
      '/grafana': stand(),
    },
  },
})
