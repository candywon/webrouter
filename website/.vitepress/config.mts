import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/webrouter/',
  title: 'WebRouter',
  description: 'Unified AI API Gateway — Manage all your LLM providers in one place',
  lang: 'en-US',

  head: [
    ['link', { rel: 'icon', href: '/webrouter/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#2563eb' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'WebRouter',

    search: {
      provider: 'local',
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/candywon/webrouter' },
    ],

    footer: {
      message: 'Released under the BSL 1.1 License. Contact: jianlin@webrouter.tech',
      copyright: 'Copyright 2026 Jianlin Huang',
    },
  },

  locales: {
    root: {
      label: 'English',
      lang: 'en',
      themeConfig: {
        nav: [
          { text: 'Home', link: '/' },
          { text: 'Docs', link: '/guide/quick-start' },
          { text: 'Demo', link: 'https://demo.webrouter.tech' },
          { text: 'GitHub', link: 'https://github.com/candywon/webrouter' },
        ],
        sidebar: {
          '/guide/': [
            {
              text: 'Getting Started',
              items: [
                { text: 'Quick Start', link: '/guide/quick-start' },
                { text: 'Installation', link: '/guide/installation' },
                { text: 'Deployment', link: '/guide/deployment' },
              ],
            },
            {
              text: 'Core Concepts',
              items: [
                { text: 'Architecture', link: '/guide/architecture' },
                { text: 'Providers', link: '/guide/providers' },
                { text: 'Tokens & Teams', link: '/guide/tokens-teams' },
                { text: 'Smart Routing', link: '/guide/smart-routing' },
              ],
            },
            {
              text: 'Operations',
              items: [
                { text: 'Monitoring', link: '/guide/monitoring' },
                { text: 'Alerting', link: '/guide/alerting' },
                { text: 'Billing', link: '/guide/billing' },
                { text: 'Desensitization', link: '/guide/desensitization' },
              ],
            },
            {
              text: 'Reference',
              items: [
                { text: 'Configuration', link: '/guide/configuration' },
                { text: 'API Reference', link: '/guide/api-reference' },
                { text: 'CLI Export', link: '/guide/cli-export' },
                { text: 'FAQ', link: '/guide/faq' },
              ],
            },
          ],
        },
      },
    },
    zh: {
      label: '中文',
      lang: 'zh-CN',
      link: '/zh/',
      themeConfig: {
        nav: [
          { text: '首页', link: '/zh/' },
          { text: '文档', link: '/zh/guide/quick-start' },
          { text: '在线演示', link: 'https://demo.webrouter.tech' },
          { text: 'GitHub', link: 'https://github.com/candywon/webrouter' },
        ],
        sidebar: {
          '/zh/guide/': [
            {
              text: '快速上手',
              items: [
                { text: '快速开始', link: '/zh/guide/quick-start' },
                { text: '安装指南', link: '/zh/guide/installation' },
                { text: '部署方案', link: '/zh/guide/deployment' },
              ],
            },
            {
              text: '核心概念',
              items: [
                { text: '架构概览', link: '/zh/guide/architecture' },
                { text: '数据源管理', link: '/zh/guide/providers' },
                { text: '令牌与团队', link: '/zh/guide/tokens-teams' },
                { text: '智能路由', link: '/zh/guide/smart-routing' },
              ],
            },
            {
              text: '运维管理',
              items: [
                { text: '健康监控', link: '/zh/guide/monitoring' },
                { text: '告警中心', link: '/zh/guide/alerting' },
                { text: '成本计费', link: '/zh/guide/billing' },
                { text: '隐私脱敏', link: '/zh/guide/desensitization' },
              ],
            },
            {
              text: '参考文档',
              items: [
                { text: '配置说明', link: '/zh/guide/configuration' },
                { text: 'API 参考', link: '/zh/guide/api-reference' },
                { text: 'CLI 导出', link: '/zh/guide/cli-export' },
                { text: '常见问题', link: '/zh/guide/faq' },
              ],
            },
          ],
        },
      },
    },
  },
})