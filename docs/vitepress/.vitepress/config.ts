import { defineConfig } from 'vitepress'

// 本 VitePress 站点位于 docs/vitepress/，运行命令时在该目录内执行 vitepress dev/build。
// 源根（root）即本目录；配置目录为默认的 .vitepress/。
//
// 图片复制到 docs/vitepress/public/images/，由 VitePress 作为静态资源直接发布。
// 因此 Markdown 中使用 /images/亮色-1.jpg 这样的站点绝对路径即可，开发、构建
// 和部署时都不依赖仓库外的绝对路径。
export default defineConfig({
  title: 'helpFly 助手',
  description: 'helpFly 桌面助手 · 项目文档',
  base: process.env.GITHUB_ACTIONS ? '/helpFly-tool/' : '/',
  lang: 'zh-CN',
  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: 'GitHub 仓库', link: 'https://github.com/gzdzh-cn/helpFly-tool' },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/gzdzh-cn/helpFly-tool' },
    ],
    sidebar: [
      {
        text: '项目文档',
        items: [{ text: '界面预览（文档站）', link: '/' }],
      },
    ],
  },
})
