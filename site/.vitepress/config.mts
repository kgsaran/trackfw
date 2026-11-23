import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/trackfw/',
  title: 'trackfw',
  description: 'Software delivery governance CLI: ADR → REQ → ROADMAP → kanban',

  head: [
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'trackfw — Governance for teams and AI agents' }],
    ['meta', { name: 'og:description', content: 'Open-source CLI for delivery governance: ADR → REQ → ROADMAP → kanban. Native support for AI agents.' }],
    ['link', { rel: 'icon', href: '/trackfw/favicon.ico', type: 'image/x-icon' }],
  ],

  locales: {
    root: {
      label: 'English',
      lang: 'en-US',
      themeConfig: {
        nav: [
          { text: 'Home', link: '/' },
          { text: 'Guide', link: '/guide/getting-started' },
          { text: 'Commands', link: '/guide/commands' },
          { text: 'AI Agents', link: '/guide/ai-agents' },
          { text: 'GitHub', link: 'https://github.com/kgsaran/trackfw' },
        ],
        sidebar: {
          '/guide/': [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Commands Reference', link: '/guide/commands' },
            { text: 'trackfw for AI Agents', link: '/guide/ai-agents' },
          ],
        },
        footer: {
          message: 'MIT License',
          copyright: 'trackfw — open-source',
        },
      },
    },
    pt: {
      label: 'Português',
      lang: 'pt-BR',
      link: '/pt/',
      themeConfig: {
        nav: [
          { text: 'Início', link: '/pt/' },
          { text: 'Guia', link: '/pt/guide/getting-started' },
          { text: 'Comandos', link: '/pt/guide/commands' },
          { text: 'Agentes de IA', link: '/pt/guide/ai-agents' },
          { text: 'GitHub', link: 'https://github.com/kgsaran/trackfw' },
        ],
        sidebar: {
          '/pt/guide/': [
            { text: 'Início Rápido', link: '/pt/guide/getting-started' },
            { text: 'Referência de Comandos', link: '/pt/guide/commands' },
            { text: 'trackfw para Agentes de IA', link: '/pt/guide/ai-agents' },
          ],
        },
        footer: {
          message: 'MIT License',
          copyright: 'trackfw — open-source',
        },
      },
    },
  },

  themeConfig: {
    socialLinks: [
      { icon: 'github', link: 'https://github.com/kgsaran/trackfw' },
      { icon: 'npm', link: 'https://www.npmjs.com/package/trackfw' },
    ],
    search: { provider: 'local' },
  },
})
