import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import {defineConfig} from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
  build: {
    // maplibre (~1 MB) and deck.gl (~680 kB) cannot be split further; they load only with
    // the map, so warn only for chunks larger than they are.
    chunkSizeWarningLimit: 1100,
    rollupOptions: {
      output: {
        manualChunks(id) {
          // Vite's dynamic-import helper is used by every lazy page. Left alone, Rollup puts
          // it in the first manual chunk that needs it (deck-runtime), and every page then
          // preloads 680 kB of deck.gl just to lazy-load a page.
          if (id.includes('vite/preload-helper') || id.includes('commonjsHelpers')) return 'runtime';
          if (!id.includes('/node_modules/')) return undefined;

          if (id.includes('/node_modules/maplibre-gl/')) {
            return 'maplibre';
          }

          if (
            id.includes('/node_modules/@deck.gl/') ||
            id.includes('/node_modules/@luma.gl/') ||
            id.includes('/node_modules/@math.gl/') ||
            id.includes('/node_modules/mjolnir.js/')
          ) {
            return 'deck-runtime';
          }

          if (id.includes('/node_modules/h3-js/')) {
            return 'h3';
          }

          if (
            id.includes('/node_modules/recharts/') ||
            id.includes('/node_modules/d3-') ||
            id.includes('/node_modules/victory-vendor/') ||
            id.includes('/node_modules/decimal.js-light/')
          ) {
            return 'charts';
          }

          if (
            id.includes('/node_modules/react/') ||
            id.includes('/node_modules/react-dom/') ||
            id.includes('/node_modules/scheduler/')
          ) {
            return 'react-vendor';
          }

          return 'vendor';
        },
      },
    },
  },
  // maplibre-gl 6 starts its worker as an ES module.
  worker: {
    format: 'es',
  },
  server: {
    hmr: process.env.DISABLE_HMR !== 'true',
    // Custom development hostnames must be explicitly allowlisted.
    allowedHosts: (process.env.VITE_ALLOWED_HOSTS ?? '').split(',').map((host) => host.trim()).filter(Boolean),
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
    },
  },
});
