import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
    title: '白虎面板',
    description: '轻量易用的定时任务面板，支持多语言脚本、依赖管理与日志查看',
    base: '/baihu-panel/',
    lang: 'zh-CN',
    themeConfig: {
        logo: '/logo.svg',
        nav: [
            { text: '快速开始', link: '/guide/introduction' },
            { text: '部署指南', link: '/guide/deployment' },
            { text: 'API 文档', link: '/guide/api' }
        ],

        sidebar: [
            {
                text: '基础指南',
                items: [
                    { text: '产品介绍', link: '/guide/introduction' },
                    { text: '部署说明', link: '/guide/deployment' },
                    { text: '开始使用', link: '/guide/getting-started' },
                    { text: 'API 文档', link: '/guide/api' }
                ]
            },
            {
                text: '使用说明',
                items: [
                    { text: '功能特性', link: '/guide/usage' },
                    { text: '编程语言与依赖管理', link: '/guide/languages' },
                    { text: '脚本示例总览', link: '/guide/examples/' },
                    { text: '浏览器示例', link: '/guide/examples/browser' },
                    { text: '命令行工具 (CLI)', link: '/guide/cli' }
                ]
            },
            {
                text: '部署配置',
                items: [
                    { text: '系统配置', link: '/guide/configuration' },
                    { text: '反向代理 (Nginx)', link: '/guide/nginx' }
                ]
            },
            {
                text: '其他',
                items: [
                    { text: '更新日志', link: '/guide/changelog' },
                    { text: '免责声明', link: '/guide/disclaimer' }
                ]
            }
        ],

        socialLinks: [
            { icon: 'github', link: 'https://github.com/engigu/baihu-panel' }
        ],

        footer: {
            message: 'Released under the MIT License.',
            copyright: 'Copyright © 2026-present engigu'
        },

        search: {
            provider: 'local'
        }
    },
    vite: {
        ssr: {
            noExternal: ['@scalar/api-reference']
        }
    }
})
