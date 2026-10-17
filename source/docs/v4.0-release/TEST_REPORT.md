# TEST_REPORT — 嘉豪工具箱 4.0.0-beta.1

**结论：当前只能交付本地评估 Beta；Windows 正式验收未完成，不能宣布 FINAL。**

构建时间（UTC）：2026-10-17T04:37:12Z。测试记录在 2026-10-17 生成。机器环境是 Linux x64 / 6.18.55，不是用户的 Windows PC。工具链 Go 1.23.2、Node.js 22.16.0、TypeScript 5.8.3、系统 Chromium + Python Playwright。

原始工具链已经过旧。缺少当前漏洞数据库扫描、受支持 Go 重建、公共 WebView2 Loader / 内部回退实测、Windows 原生与 GUI 稳定性验收，均属于发布阻断项，详见 RELEASE_BLOCKERS.md。

## 1. 实际执行与结果

| 检查 | 实际结果 | 范围 |
|---|---|---|
| `go test -count=1 -json -coverprofile=… ./...` | **43 项测试通过，0 失败** | Linux 本机构建所包含的测试；不含 Windows 专用文件的执行 |
| `go test -race -count=1 ./...` | **通过** | Linux race 检查；无报告的数据竞争；不是 Windows COM 线程安全实证 |
| `go vet ./...` | **通过** | Linux 静态分析 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...` | **通过** | Windows 目标静态分析，未执行 |
| Go 语句覆盖率 | **64.3%** | 仅本次 Linux 编译进入覆盖统计的代码；不能作为 Windows 采集器覆盖率 |
| `node scripts/build-frontend.mjs` | **通过** | TypeScript 严格编译，静态资源生成 |
| `node --test tests/frontend.test.mjs` | **12 项通过，0 失败** | 单位、空值、XSS 转义、时间、图表单位/序列、60 秒窗口等 |
| Chromium 离线界面矩阵 | **38 个布局检查通过** | 28 组分辨率/比例 + 最小有效窗口下其余 10 页 |
| Chromium 交互 | **26 项通过，0 个 JS 错误** | 页面、搜索、设置持久化、真实暂停/恢复、真实诊断、导出确认/取消、空搜索等 |
| Windows x64 GUI EXE | **交叉编译成功** | PE32+ / AMD64 / GUI，**未在 Windows 运行** |
| Windows 目标测试二进制 | **6 个编译成功，0 个执行** | monitor、native、systemtools、config、app、hardware；ABI 断言未运行 |
| 随包源码同参数重建 | **通过，EXE 逐字节一致** | 同 Go 1.23.2、相同时间/version ldflags、trimpath/buildvcs；不代表跨工具链复现 |
| PE 资源检查 | **通过** | 实际 EXE 含 PMv2/asInvoker manifest、图标组和7个图标尺寸；不是 DPI 实机测试 |
| Linux 后端只读自检 30 秒 | **完成** | 实际 30 个样本；硬件状态 partial 如实保留 |
| 至少 1 小时完整 GUI / Windows 稳定性 | **未执行** | 提供可复現设计和脚本，没有伪造通过记录 |
| 在线漏洞库扫描 / Authenticode | **未执行 / 未签名** | 不宣称“零漏洞”“安全认证” |

Go 单元测试使用专门的边界输入/样本作为断言，不把这些测试值注入生产 UI；生产与截图使用实际运行后端或明确不可用值。

## 2. UI 测试口径与矩阵

受管理 Chromium 的 URL 导航被策略拒绝（ERR_BLOCKED_BY_ADMINISTRATOR）。未修改策略。测试使用 `about:blank` 离线文档、编译后的原有 ES 模块/CSS/SVG，并将测试文档内 fetch 通过 Python 转发至正在运行的 Go 回环 HTTP 服务。因此 **数值和诊断是实际后端读取结果，不是随机模拟数据**；但不能证明实际 WebView2 导航、浏览器 Cookie 持久化、原生下载、剪贴板或跨屏 DPI。

分辨率：1366×768、1920×1080、2560×1440、3840×2160。

比例：100%、125%、150%、175%、200%、250%、300%。CSS viewport 使用物理尺寸除以比例向下取整，同时设定 deviceScaleFactor；例如 1366×768/300% 对应 455×256 CSS px。检查 body/main/topbar 横向溢出和可操作元素越界，卡片内容可以按设计纵向滚动。

28 组均通过；额外在最窄窗口检查 CPU、GPU、内存、磁盘、网络、硬件、工具、诊断、设置、关于 10 页。深色矩阵与浅色 1920×1080检查通过。没有因此声称 Windows 真实系统的 28 组 DPI 已通过。

`source/tests/ui_renderer.py` 和 `ui_matrix.py` 是完整复现脚本；`source/docs/TEST_EVIDENCE.json` 保存逐组结果。导出测试覆盖确认和 Escape 取消，没有测试在 Windows 上实际保存文件。网络界面测试没有向外网发送探测。Linux 页面明确禁用 Windows 管理工具，本次没有验证其 Windows 真实启动结果。

## 3. 实际测量，不套用 Windows 性能结论

| 测量 | 本次观测 | 不能推导的结论 |
|---|---|---|
| 11 页本地 DOM 路由到下一帧 | 10.2–56.3 ms，单轮 Linux 离线渲染观测 | 不是 Windows 页面切换 P95，也不包含完整系统渲染链路 |
| Linux 只读自检墙钟 | 30.001 s，30 个样本 | 不是一小时稳定性测试 |
| 最后一次采样耗时 | 0.458 ms | 单个 Linux 样本，不是平均值或 Windows PDH/CIM/GPU 开销 |
| Go heap 开始/结束 | 402,624 / 1,895,832 bytes | 仅 Go heap，不是完整工作集，更不是含 WebView2 的总内存 |
| Go goroutine 开始/结束 | 6 / 5 | 一次短测试，不能证明无长期泄漏或 Windows 子进程残留 |

Windows 冷启动 <3s、热启动 <1.5s、空闲 CPU <1%、完整进程树内存 <150MB、页面 <200ms：**均没有实测结论**。不拿开发工具启动时间、端口监听时间或只有 Go heap 的值填入这些目标。

## 4. 开发阶段实际记录

| 阶段 | 已完成文件/实现 | 已执行验证 | 尚需下一步 |
|---|---|---|---|
| Phase 1 基础架构 | cmd/jiahao-toolbox、internal/native/app/config/logging、frontend/embed、resourcegen、manifest | Linux 后端运行；Windows 编译/vet/PE检查；会话/配置/日志/race测试 | Windows 空白/完整宿主启动退出、COM 生命周期、公共 Loader |
| Phase 2 UI | frontend/src main/components/charts/styles/services/assets，11页与深浅主题 | TypeScript、12前端单元、38布局、26交互、截图查看 | 写实横幅与参考图精细对齐；Windows 原生 UI / DPI |
| Phase 3 真实监控 | internal/monitor 的 CPU/内存/磁盘/网卡/PDH/DXGI/GPU；hardware CIM | Windows 代码编译和计算测试；Linux真数据，自检30秒 | Windows指标、驱动差异、普通用户权限和数值核对 |
| Phase 4 工具诊断 | systemtools固定10入口、startup、Ping/DNS；diagnostics | 白名单/参数/评分测试；Linux诊断和确认交互 | 实际 Windows 外部工具、UAC、复制保存、启动快捷方式 |
| Phase 5 优化 | 统一采样、121项上限、降频、异步缓存、取消、日志轮转、实例锁、SVG | race/配置/超频防护/无效值/日志边界、渲染布局 | 完整进程树资源、长时运行、异常恢复、真实跨屏 |
| Phase 6 测试交付 | EXE、source、SHA256、BUILD、测试与Windows验收脚本 | 本报告列出的实际测试和打包检查 | RELEASE_BLOCKERS清单完成后才能升级稳定版 |

## 5. 已修复的问题

空 GPU 数组绘图异常、页面跳转属性不一致、未知字段/非对象请求接受风险、窗口消息触发重复采样、超大单次日志超限、多实例争用配置、空硬件搜索反馈、单位 SI kB 标识、多处理器组总体 CPU 回退范围，以及 Windows 窗口消息结构的静态指针检查。修复后重新执行相关单元/编译/渲染检查。没有把未执行的 Windows 运行错误“修复验证”写入记录。

## 6. 交付文件证据

EXE：`JiahaoToolbox_v4.0.exe`，6,482,944 bytes（6.18 MiB），Go 1.23.2，Windows AMD64 GUI。

```text
SHA256 00932dbb67300ca4d95b9bb5122ad54b999203f1066b2c2867eef87f0e65a1f9
```

该哈希只对应本次评估 EXE，不对应后续重新构建或可选公共 Loader。完整文件哈希见 SHA256SUMS.txt。便携 ZIP 不包含临时测试 profile、原始硬件自检数据、开发用 Linux 二进制或 Windows test.exe。

**结论保持 Beta。** 所有 NOT RUN 项必须在实际 Windows 环境补测；若失败应记录并修复，不能自动把本报告复制成下一版通过证明。
