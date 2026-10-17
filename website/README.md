# 嘉豪工具箱官方网站

独立静态官网，使用 HTML5、CSS3、TypeScript、Vite。深蓝黑风格，响应式桌面/平板/手机布局。没有数据库、登录、广告、分析追踪或外部字体。

官网部署目标：https://stone2013.github.io/JiahaoToolbox/ 。初版本地验收已完成，维护者已明确确认上线；当前部署状态以仓库 GitHub Actions 的 Pages 运行记录为准。

## 本地运行

需要 Node.js 22.12+（开发与工作流使用 Node.js 24）。

```powershell
cd website
npm ci --ignore-scripts
npm run dev
```

浏览器访问终端地址加 `/JiahaoToolbox/`，默认 http://127.0.0.1:4174/JiahaoToolbox/ 。

```powershell
npm run build
npm run check:assets
npm run preview
```

`build` 在构建阶段请求固定官方 GitHub latest Release API，然后执行 TypeScript 检查与静态构建，产物位于 `dist/`。`build:offline` 使用已核验的 `src/release.json` 快照，不访问 API。页面本身不轮询 API；下载和仓库按钮只使用官方地址。

## 测试

```powershell
npm run test:unit
npx playwright install chromium
npm run test:e2e
```

可以用 `PLAYWRIGHT_CHROMIUM_EXECUTABLE` 指定本机已有 Chromium 的可执行文件。浏览器测试使用桌面、平板和手机尺寸；手机为 Chromium 触控/DPR 模拟，不代表真实 Safari/iOS 实机验收。运行测试前先构建。当前结果见 [TEST_REPORT.md](TEST_REPORT.md)。

## 内容和资源

- `index.html`：首页、10项功能、截图、实录视频与下载区。
- `src/style.css` / `src/main.ts`：响应式布局、手机导航、截图对话框、播放按钮、轻量滚动动效及减少动画支持。
- `src/release.json`：最近一次官方 Release 的已核验快照，版本、时间和附件体积没有硬编码为界面示例。
- `scripts/release-data.mjs`：固定仓库校验、稳定版筛选、官方附件链接校验、404与网络失败处理。
- `public/images/`：来自真实软件录屏的 WebP 截图、响应式尺寸及视频封面。
- `public/media/`：本地压缩版90秒实录宣传片，1080p60、H.264/AAC48kHz，约20.8MB。`preload="none"`，用户主动播放后才加载。
- `404.html`、`robots.txt`、`sitemap.xml`：基础SEO及静态404。
- `docs/ASSET_PROVENANCE.json` / [ASSETS.md](docs/ASSETS.md)：真实素材来源、裁切和隐私说明。
- `../.github/workflows/pages.yml`：独立 Pages 构建/部署工作流，不改动桌面软件工作流。

截图数值为录制时状态，不是网页上的实时监控。设备名与个人字段沿用已审核的不透明遮挡。软件当前未签名、安装测试范围有限，下载区保留相关说明。没有宣称自动安装更新。

版本信息来自官方 Releases。404时显示“正式版本即将发布”，不展示下载量或虚构版本。API限流/断网时保留最近一次已核验快照及其同步日期，官方下载页始终可用。

## 修改与部署

文案和结构修改 `index.html`，样式修改 `src/style.css`。替换截图时应使用真实界面、保持隐私遮挡并更新素材追溯。不要把低清图放大为高分辨率。

部署操作和需要确认的远程改动见 [DEPLOYMENT.md](DEPLOYMENT.md)。本项目没有调用 Sites 托管或第三方视频平台，目标是用户指定的 GitHub Pages。
