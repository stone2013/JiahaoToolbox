# 架构、通信与生命周期

## 宿主选择

首选要求是 Go + WebView2，并非 Electron。环境有 Go x64 交叉编译器，但无法通过 shell DNS 拉取第三方封装依赖，不能在 Windows 上先做空白应用实机验证。为实际交付，当前采用标准库 syscall 的原生窗口和 COM 封装，标记为实验 Beta；没有宣称已证明它优于成熟封装。

原生窗口在锁定 OS 线程的 COM STA 创建。异步硬件查询、采样、本地 HTTP 请求在独立任务执行，窗口按钮以 PostMessage 送回 GUI 线程。WebView2 控件创建、事件注册、尺寸更新与释放留在 STA。PMv2 manifest + SetProcessDpiAwarenessContext + WM_DPICHANGED；图标组含 16/24/32/48/64/128/256 像素资源，网页图标/图表为 SVG。

运行时尝试公共 Loader，缺省的单 EXE 内部入口回退仍未实机验证。正式发布应替换为公共/官方静态 Loader 并验证消息/窗口/DPI，不以减少一个 DLL 为理由忽视稳定性。

## 本地桥

前端 ES 模块、CSS、SVG 嵌入 EXE。原生 WebView2 访问随机回环端口，随机 bootstrap URL 设置 HttpOnly SameSiteStrict 会话 Cookie，再重定向到根页面。Cookie 按实例分隔；除 bootstrap 外静态资源也需会话。Host、Origin、Sec-Fetch-Site、写请求标识、方法和 JSON 字段逐层校验。接口不是任意命令桥，不接受任意可执行文件、Shell 脚本或写入路径。

前端默认每秒取一次最新状态和有限历史。请求有 AbortController 超时/取消；后台查询已缓存，前端轮询不是再次查询整套 CIM。硬件刷新独立为异步操作，可取消其 context 和 PowerShell 子进程。联网检查有超时且必须确认。

## 采样

唯一 monitor.Service 启动一次，依次调用 Collector，不重入。最短 500ms，后台至少 5s。配置完全相同不发送重复唤醒；间隔计时避免窗口消息导致超频采集。暂停不追加采样，恢复按实际经过时间计算速率。历史最多 121 个 Snapshot；图表只显示最近 60 秒，缓冲区不是无限 append。

后端保留状态值的 unit/status/reason。无首个差分、计数器重置、异常浮点、零分母和不可用接口返回 null，而不是取 0 或随机数。图表不跨无效点画伪连续线，不因“平滑”修改读数。

Windows：GetSystemTimes+PDH、GlobalMemoryStatusEx、GetDiskFreeSpaceEx、GetVolumeInformation、GetIfTable2/FreeMibTable、DXGI、WDDM 性能计数器。系统硬件信息走固定的可信 PowerShell/CIM 脚本，NoProfile/NonInteractive，有输出长度、总超时和单类超时；不根据用户输入拼接命令。

GPU 同一物理引擎的进程占用求和，然后以最忙引擎作为适配器指标，不把不同引擎相加超过 100%。LUID 与 DXGI 适配器对应；需要实机验证多 GPU、链接适配器、驱动/计数器差异。

## 退出、缓存、失败

根 context 取消 -> HTTP shutdown -> 采样任务结束/Close -> 硬件查询取消/Wait -> 日志关闭 -> 配置锁释放。原生宿主还需回收 Controller、COM 引用、事件回调/Pin、窗口和 STA。Windows 执行链不能在 Linux 静态测试中证明，单列验收项。

单个硬件类失败不使其他类消失；provider Go panic 转为本次采样错误。Windows API 的原生调用并不全部具备可抢占超时，极端驱动卡死尚未验证。这是隔离/稳定性测试必须覆盖的内容，不能仅凭 context 字段声称所有 API 都可取消。

容量缓存、网卡地址缓存 5s；GPU 枚举 30s；硬件详细信息启动异步读取并按需刷新。固定接口/分区/适配器数量有界，用户可主动刷新。浏览器 Runtime 自身缓存不受 Go ring/log 大小限制，需要另外测量。

## 当前验证范围

Linux 真数据验证 + Go race + TypeScript/格式化/图表单元测试 + 本地 HTTP 认证测试 + Chromium 离线渲染矩阵 + Windows 目标编译/vet/PE 资源检查。不包含 Windows API 运行、原生 WebView2 网络导航/下载、DPI 跨屏和用户机器性能；详见 TEST_REPORT。
