import { paraglideVitePlugin } from '@inlang/paraglide-js'
import { sveltekit } from '@sveltejs/kit/vite'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [
    tailwindcss(),
    sveltekit(),
    paraglideVitePlugin({
      project: './project.inlang',
      outdir: './src/lib/paraglide',
    }),
  ],
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': 'http://localhost:8080',
      '/share': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/config': 'http://localhost:8080',
      '/demo': 'http://localhost:8080',
      '/integrations': 'http://localhost:8080',
    },
  },
})
