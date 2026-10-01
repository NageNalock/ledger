import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import { readFileSync } from 'node:fs'

const agentGuide = () => readFileSync(path.resolve(__dirname, 'src/content/agent-access.html'), 'utf8')
import Inspector from 'unplugin-vue-dev-locator/vite'

// https://vite.dev/config/
export default defineConfig(({ command }) => ({
  build: {
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            return 'vendor'
          }
        },
      },
    },
  },
  plugins: [
    vue(),
    {
      name: 'ledger-public-agent-guide',
      generateBundle() {
        this.emitFile({
          type: 'asset', fileName: 'agent.html',
          source: '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ledger · Agent 接入</title><link rel="alternate" type="text/markdown" href="/SKILL.md"></head><body style="margin:0;padding:32px 16px;background:#fffaf6"><main>' + agentGuide() + '</main></body></html>',
        })
      },
    },
    command === 'serve' ? Inspector() : null,
  ].filter(Boolean),
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
}))
