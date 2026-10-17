# CHANGELOG

## v4.4.0 — Windows 安装与安全更新检查

- 增加统一 `version.json`，驱动 Go linker、Win32 VERSIONINFO 资源、前端启动/主界面版本、Inno Setup、Portable/Setup 文件名和 GitHub 标签约定。
- 新增 Windows x64 Inno Setup 简体中文安装器：当前用户安装为默认、可选全用户/Program Files 安装、检测已有版本并复用安装目录、桌面及开始菜单快捷方式、覆盖升级、启动选择。卸载默认保留个人数据，卸载流程可另选删除应用自己的数据目录。
- 检测 WebView2 Runtime 用户级/机器级注册表项。缺失时不阻塞安装、不捆绑 Runtime，提供打开 Microsoft 官方下载页面的选择。
- 新增官方仓库 GitHub API 更新检查，包含 SemVer/预发布比较、当前/最新版本、发布日期、更新日志、附件名称与体积、检查时间/渠道，处理无 Release、网络错误与 API 限流。只允许固定官方 API 与 Release 页面。
- 设置更新检查、启动检查和测试版渠道均默认关闭。自动下载控件后端拒绝开启：当前构建没有预置签名公钥或 Authenticode 证书，不把同一 Release 的 SHA256 当成发布者身份验证，不自动下载/执行/安装。
- 修复绿色 CMD 启动可靠性：使用 Windows SystemRoot\System32\cmd.exe 和独立新控制台；所选目录只作为工作目录传入，不拼接进 shell 命令；系统目录缺失或启动失败时明确报错。
- 新增 PR/标签/手动触发 GitHub Actions：构建测试、x64 EXE、安装器、便携 ZIP 与 SHA256；正式 Release 仅允许在 `workflow_dispatch` 中明确选择发布且 ref 与 version.json 完全匹配后创建。没有代码签名密钥时产物明确标记未签名。
- 保留 v4.3 及之前的硬件监控、动画、拼音搜索、网络/磁盘测速、经典 CMD、主题与启动效果。

## v4.3 — 2026-10-17

- 新增可取消的真实 HTTP 网络测速、文件型磁盘基准测试和经典 CMD；详见上版测试报告。

## v4.2 — 2026-10-17

- 分离真实硬件采样与监控显示插值，增加帧动画和曲线平滑。

完整历史和旧版验证边界见源码与 `docs/v4.0-release/`。
