# 构建 v4.4.0

前置工具：Windows x64、Go 1.27.2（与 `version.json` 对齐）、Node.js 24、npm、Inno Setup 6。程序本身依赖 WebView2 Runtime，但 Runtime 不会打进安装器。

在 `source` 执行：

```powershell
npm --prefix frontend ci --ignore-scripts
.\scripts\build-release.ps1 -Output ..\JiahaoToolbox_v4.4.0
```

脚本从 `version.json` 生成前端、Go/Windows GUI EXE、Win32 VERSIONINFO、安装包、Portable ZIP 和 SHA256 清单。当前没有 Authenticode 证书；产物未签名。SHA256 检测传输损坏，不验证发布者身份。

单项检查：

```powershell
$env:PATH = "C:\\path\\to\\go\\bin;$env:PATH"
go test -count=1 ./...
go vet ./...
npm --prefix frontend run typecheck
node scripts/build-frontend.mjs
node --test tests/frontend.test.mjs tests/search.test.mjs tests/search-ui.test.mjs tests/startup.test.mjs tests/monitor-animation.test.mjs
```

本地 Release 目录脚本不会推送 GitHub。Workflow 位于 `.github/workflows/release.yml`：PR 做测试构建；tag 产生构建附件但不自动发布；正式发布要求手动运行 workflow_dispatch、选中 `publish_release` 且标签等于 version.json。
