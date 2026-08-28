import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Production build assumes the app is served by `mg serve` (same origin, no
// proxy needed in dev against a real daemon on localhost:8080 thanks to the
// server.cors-less fetch fallback). The mock mode (--mode mock) swaps the API
// layer for an in-browser fixture backend — see src/lib/api/mock.ts.
export default defineConfig(({ mode }) => ({
  plugins: [svelte()],
  resolve: {
    alias: {
      $lib: new URL('./src/lib', import.meta.url).pathname,
    },
    // Component tests need the browser build of Svelte, not the server one.
    conditions: ['browser'],
  },
  define: {
    // The mock backend is compiled in but inert unless explicitly enabled —
    // either the mock mode flag or ?mock=1 on the URL.
    __MG_MOCK__: mode === 'mock',
  },
  server: {
    port: 5173,
    // Talking to a real `mg serve` on localhost:8080 from the vite dev
    // server needs no proxy when the API client is given an absolute base
    // URL (the connection setting); the proxy is a convenience for
    // same-origin development.
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        rewrite: (path) => path.replace(/^\/api/, ''),
        // mg serve is tokenless on localhost by default; the browser fetch
        // carries the bearer token from the connection setting when set.
      },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    setupFiles: ['src/test-setup.ts'],
    css: false,
  },
}))
