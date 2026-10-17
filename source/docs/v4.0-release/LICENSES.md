# 许可证与第三方组件

本表是当前构建所用组件/素材的归属记录，不等于专业法律意见或完整安全审计结论。未声称审查通过所有漏洞。

| 组件/资源 | 版本/范围 | 许可与分发情况 |
|---|---|---|
| Jiahao Toolbox 源码 | 4.0.0-beta.1 | MIT；完整文本见 LICENSE |
| Go runtime / 标准库 | 原始 EXE：Go 1.23.2 | Go BSD 风格许可；resources/licenses 中保留原始文本；编译进 EXE；工具链过旧是发布阻断项 |
| TypeScript | 5.8.3 | Apache-2.0；仅开发编译器，不作为运行包分发；保留 LICENSE 文本 |
| Microsoft WebView2 Runtime | 用户机器实际安装版本 | Microsoft 单独授权；未打包、未安装、未下载，不属于 MIT 授权范围 |
| Microsoft WebView2Loader.dll | 可选，未附带 | 公共 SDK Loader 的再分发应遵守所取得的 Microsoft SDK 许可并保留版本和哈希；不能用本项目 MIT 文本替代 |
| Windows API、PowerShell、CIM、PDH、DXGI | 操作系统提供 | 系统组件未分发；本项目编写接口调用，不复制第三方 Go 封装实现 |
| 标识、图标、雪山湖泊 SVG、由标识生成的 ICO | frontend/src/assets、components/icons.ts、resources/app.ico | 原创图形，项目贡献方将可许可权利按 CC0-1.0 提供；保留 CC0 文本 |
| 系统字体 | Segoe UI Variable / Segoe UI / Microsoft YaHei / 回退 | 只通过 CSS 请求已安装字体，未附带任何字体文件 |
| Python Playwright | 可选测试依赖 | 本包不分发其程序；仅附带本项目测试脚本，使用者自行按官方许可安装 |

许可文本位置：源码目录 `resources/licenses/`；便携包对应 `source/resources/licenses/`。

没有 Electron、第三方 Go 模块、远程脚本、CDN 字体、图标包或网络图片。第三方 Go WebView2 封装曾作为方案候选评估，但由于构建环境不能取得依赖，未将其代码打包。本项目的自写 COM 桥仍需 Windows ABI/生命周期实机验收，不因“自写”就视为更可靠。

参考图仅用于用户指定的布局对照，未把用户上传的参考图作为产品横幅或分发素材。

安全记录：固定前端开发依赖版本、Go 模块清单检查、请求/命令白名单及输入校验测试已经进行；在线漏洞数据库扫描、二进制签名、供应链来源自动验证和 Windows 运行安全测试没有完成。

官方说明：
- Go 发布支持周期：https://go.dev/doc/devel/release
- TypeScript 仓库许可：https://github.com/microsoft/TypeScript/blob/main/LICENSE.txt
- WebView2 SDK / Runtime：https://learn.microsoft.com/en-us/microsoft-edge/webview2/
- CC0 文本：https://creativecommons.org/publicdomain/zero/1.0/legalcode
