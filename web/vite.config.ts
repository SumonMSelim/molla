/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

function devCsp() {
  return {
    name: 'molla-dev-csp',
    transformIndexHtml(html: string, ctx: { server?: unknown }) {
      if (!ctx.server) {
        return html
      }
      return html.replace(
        /content="default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self';/,
        "content=\"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:;",
      )
    },
  }
}

export default defineConfig({
  base: '/app/',
  plugins: [react(), tailwindcss(), devCsp()],
  server: {
    proxy: {
      '/api': {
        target: process.env.API_PROXY ?? 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
  },
})
