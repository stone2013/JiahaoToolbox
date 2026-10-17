# GitHub Pages 部署说明

## 状态与确认范围

初版已完成本地源码、构建产物和测试。维护者已明确回复“上线”，授权上传网站、合并 main、配置 GitHub Actions 作为 Pages 来源并执行首次部署。实际线上状态以 Actions 和线上验收为准。

以下为本次已获确认的部署范围；未来扩大范围仍需确认：

1. 向现有 `stone2013/JiahaoToolbox` 上传新增 `website/` 与 `.github/workflows/pages.yml`，建议先建立网站分支供审阅，再合并到 `main`。
2. 在仓库 Settings → Pages 将 Build and deployment 的 Source 设置为 **GitHub Actions**；若已有网站或其他设置，先核实，不能覆盖已有内容。
3. 合并后手动触发一次 Official website — GitHub Pages，或由 main 上对应路径的提交触发首次部署。

不修改桌面软件 `source/`、既有 `.github/workflows/release.yml` 或已发布的 Release，不新增其他仓库。

## 工作流行为

- PR：构建、检查 TypeScript、单元测试、浏览器测试并上传 Pages 构建产物，**不部署**。
- main 上仅 `website/**` 或本工作流变化：测试后部署。
- 正式 GitHub Release published：检出默认分支，重新获取最新软件版本并构建/测试/部署官网。
- workflow_dispatch：允许维护者手动构建/部署默认分支。
- 构建失败不进入部署。仅部署 job 获得 `pages:write`、`id-token:write`，使用 OIDC；不需要把访问令牌提交到源码。
- 官方 checkout、setup-node、configure-pages、upload-pages-artifact、deploy-pages 以已核验的完整提交SHA固定；见 `docs/action-pins.json`。
- 工作流未使用 enablement 参数，不在后台自动开启或重设仓库 Pages；需要确认后配置 Source。

官方流程依据：[GitHub 自定义 Pages 工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)、[Vite 的 GitHub Pages 部署](https://vite.dev/guide/static-deploy.html#github-pages)。

## 路径与构建

Vite `base` 为 `/JiahaoToolbox/`。构建入口为首页及 `404.html`，输出 `website/dist/`。图片、视频、图标、CSS、JS和404返回地址均使用同一子路径。

```powershell
cd website
npm ci --ignore-scripts
npm run build
npm run check:assets
npm run test
```

CI通过 Playwright 官方CLI安装 Chromium；本地可以指定已有 Chromium。Node.js 24，静态资源无需生产服务器或数据库。

## 部署后验收

确认工作流成功并得到实际 page_url 后，再检查：

- https://stone2013.github.io/JiahaoToolbox/ 返回正常页面。
- CSS/JS/图片在 `/JiahaoToolbox/` 下无404。
- 真实手机、平板和桌面显示正常；截图点击放大，Esc可关闭。
- 宣传片可手动播放且能听到声音；无首屏自动下载或自动播放。
- 下载只指向官方 GitHub Releases；版本/发布日期与官方 API 一致。
- 访问不存在的URL会呈现自定义404，返回首页链接正确。

以上线上验收在尚未部署时不得记为通过。

## 回退

如果上线后出现问题，可恢复前一次成功的网站提交并重新部署，或通过 GitHub Pages 的部署管理恢复先前已验证产物。当前尚无本网站的线上部署，因此未实际测试回退；不能宣称已具备验证过的自动回滚能力。
