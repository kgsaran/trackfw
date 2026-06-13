import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/trackfw/',
  title: 'trackfw',
  description: 'CLI de governança para entrega de software: ADR → REQ → ROADMAP → kanban',

  head: [
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'trackfw — Governança para times e agentes de IA' }],
    ['meta', { name: 'og:description', content: 'CLI open-source para governança de entrega: ADR → REQ → ROADMAP → kanban. Suporte nativo a agentes de IA.' }],
    ['link', { rel: 'icon', href: '/trackfw/favicon.ico', type: 'image/x-icon' }],
  ],

  locales: {
    root: {
      label: 'Português',
      lang: 'pt-BR',
      themeConfig: {
        nav: [
          { text: 'Início', link: '/' },
          { text: 'Guia', link: '/guide/getting-started' },
          { text: 'Comandos', link: '/guide/commands' },
          { text: 'IA Agents', link: '/guide/ai-agents' },
          { text: 'GitHub', link: 'https://github.com/kgsaran/trackfw' },
        ],
        sidebar: {
          '/guide/': [
            { text: 'Início Rápido', link: '/guide/getting-started' },
            { text: 'Referência de Comandos', link: '/guide/commands' },
            { text: 'trackfw para Agentes de IA', link: '/guide/ai-agents' },
          ],
        },
        footer: {
          message: 'MIT License',
          copyright: 'trackfw — open-source',
        },
      },
    },
    en: {
      label: 'English',
      lang: 'en-US',
      link: '/en/',
      themeConfig: {
        nav: [
          { text: 'Home', link: '/en/' },
          { text: 'Guide', link: '/en/guide/getting-started' },
          { text: 'Commands', link: '/en/guide/commands' },
          { text: 'AI Agents', link: '/en/guide/ai-agents' },
          { text: 'GitHub', link: 'https://github.com/kgsaran/trackfw' },
        ],
        sidebar: {
          '/en/guide/': [
            { text: 'Getting Started', link: '/en/guide/getting-started' },
            { text: 'Commands Reference', link: '/en/guide/commands' },
            { text: 'trackfw for AI Agents', link: '/en/guide/ai-agents' },
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
    logo: { src: '/trackfw/logo.svg', alt: 'trackfw' },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/kgsaran/trackfw' },
      { icon: 'npm', link: 'https://www.npmjs.com/package/trackfw' },
    ],
    search: { provider: 'local' },
  },
})
