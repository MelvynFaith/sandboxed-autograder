import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    testTimeout: 10_000,
    // Used by `npm test -- --coverage` (CI). NFR-07 asks for at least 70%; these sit a few points
    // below the measured 86% statements, 82% branches, 90% functions and 87% lines.
    coverage: {
      provider: 'v8',
      include: ['src/**'],
      thresholds: { statements: 80, branches: 75, functions: 85, lines: 80 },
    },
  },
})
