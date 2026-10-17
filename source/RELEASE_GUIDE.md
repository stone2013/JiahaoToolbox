# v4.4.0 发布指南

仓库固定为 `stone2013/JiahaoToolbox`。正式版本使用 `v` + `version.json` 中的 SemVer，例如 `v4.4.0`。正式 Release 不由每次提交触发。

## 本地构建与验证

先完成 `BUILD.md` 的构建与测试。检查安装器、便携包和 `SHA256SUMS.txt`，并在独立 Windows 测试账户/虚拟机执行首次安装、当前用户安装、全用户安装、覆盖升级、卸载保留/删除数据与 WebView2 缺失场景。当前开发环境的测试结果和未测项记录在 `TEST_REPORT.md`。

## GitHub Actions

工作流在 PR、`v*` 标签和手动 dispatch 上构建测试产物。普通提交不会创建正式 Release。即使推送 tag，也只生成 workflow artifact。正式发布必须由有权限的人在 Actions 手动运行 workflow_dispatch、明确勾选 `publish_release`，并选择与 `source/version.json` 完全一致的 `vX.Y.Z` tag。发布 job 使用仓库 `GITHUB_TOKEN`，不会把令牌写入源码。

## 更新安全边界

当前安装器与 EXE 未做 Authenticode 代码签名，也没有已配置的离线可信更新公钥。因此应用只读固定 GitHub API，并可打开固定官方 Releases 页面让用户手动检查/下载；不自动下载、替换、运行或安装 Release 资产。SHA256 清单仅供传输完整性核对，不证明发布者身份。切勿通过自签的同一 Release checksum 启用静默更新。

要启用受验证的下载与升级，维护者需先离线生成签名密钥对，将私钥作为 GitHub Secret 安全保存、将公钥固定进源码，再设计带版本/架构/资产摘要的签名清单、撤销/轮换策略和升级辅助程序，并通过安全审查与回滚测试。此项当前未实现。

## WebView2

WebView2 Runtime 单独由 Microsoft 提供。安装器只检查注册表中的 Runtime 版本；缺失时由用户选择打开微软官方页面，不静默下载、不执行第三方组件。
