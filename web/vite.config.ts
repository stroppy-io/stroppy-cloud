import { readdir, readFile, stat, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { brotliCompressSync, constants, gzipSync } from 'node:zlib'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin, type ProxyOptions } from 'vite'
import tokens from './src/kit/tokens.json'

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

// The page is blank until the app chunk runs, on a slow link for seconds. Paint the theme's canvas
// from the first byte, picking the mode the app will (stored choice, else the system's).
function bootCanvas(): Plugin {
  const { dark, light } = tokens.color
  const script = `(function(){var m;try{m=localStorage.getItem('stroppy.theme')}catch(e){}if(m!=='dark'&&m!=='light'){m=window.matchMedia&&window.matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'}var c=m==='light'?'${light.canvas}':'${dark.canvas}';document.documentElement.style.background=c;document.documentElement.style.colorScheme=m})()`
  return {
    name: 'stroppy-boot-canvas',
    transformIndexHtml: () => [{ tag: 'script', children: script, injectTo: 'head' }],
  }
}

// The server embeds dist/ and sends a .br or .gz sibling when the browser takes it; the stdlib has
// no brotli, so the files are compressed once here.
function precompress(): Plugin {
  const worth = /\.(js|mjs|css|html|svg|json|map|txt|wasm|ttf)$/
  let outDir = ''
  return {
    name: 'stroppy-precompress',
    apply: 'build',
    configResolved: (config) => {
      outDir = path.resolve(config.root, config.build.outDir)
    },
    async closeBundle() {
      for (const name of await readdir(outDir, { recursive: true })) {
        const file = path.join(outDir, name)
        if (!worth.test(file) || (await stat(file)).size < 1024) continue
        const raw = await readFile(file)
        const br = brotliCompressSync(raw, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } })
        const gz = gzipSync(raw, { level: 9 })
        if (br.length < raw.length * 0.9) await writeFile(`${file}.br`, br)
        if (gz.length < raw.length * 0.9) await writeFile(`${file}.gz`, gz)
      }
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
    bootCanvas(),
    precompress(),
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
