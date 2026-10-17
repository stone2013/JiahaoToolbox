# 构建与复现

## 原始交付构建的事实

原始 EXE 在 Linux x64 上使用 Go 1.23.2、TypeScript 5.8.3、Node.js 22.16.0 交叉编译。不存在第三方 Go 模块依赖，`CGO_ENABLED=0`。使用了 Go 标准库的 Windows syscall 与自行编写的 Win32/WebView2 COM 桥。

该 Go 工具链已经过旧，不能把构建成功等同于安全通过。默认构建脚本拒绝低于 Go 1.26 的版本；这只是静态最低门槛，不代表此版本在未来仍受支持。发布者需要自行核查当前官方支持周期、安装对应分支最新安全修订版，并进行漏洞扫描。

官方发布记录：https://go.dev/doc/devel/release
官方漏洞扫描说明：https://go.dev/doc/tutorial/govulncheck

本环境无法联网获取新版工具链或进行在线漏洞数据库扫描。未执行的扫描不得填为“无漏洞”。

## 最少步骤：从随包已编译前端构建

Windows x64 开发机安装当前受支持且已修补的 Go 后，在 `source/` 打开 PowerShell：

```powershell
.\scripts\build.ps1
```

输出：`build/JiahaoToolbox_v4.0.exe`。随包已有 `frontend/dist/`、资源图标和纯 Go COFF 资源生成器，**不修改前端时无需 Node.js 或 TypeScript**。脚本不自动修改 PowerShell 执行策略，不自动下载工具链或 SDK。组织策略禁止脚本时，可按下面的底层命令手动构建；不要为了构建关闭系统安全策略。

```powershell
go run ./cmd/resourcegen
$env:GOOS='windows'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -trimpath -buildvcs=false -ldflags '-s -w -H=windowsgui' -o build/JiahaoToolbox_v4.0.exe ./cmd/jiahao-toolbox
```

底层命令不进行版本检查，也不写正式构建时间；应优先使用脚本。清除 GOOS/GOARCH 或打开新终端后再运行本机测试。不能在 Linux 上把 `GOOS=windows go test` 的交叉编译结果称为已执行测试。

## 修改前端

需开发环境 Node.js 和 TypeScript 5.8.3。前端没有任何运行时 npm 依赖。已固定 `package.json` 版本，但没有完成 npm 注册表漏洞审计，未附带由在线解析伪造的锁文件。

```powershell
cd frontend
npm install --ignore-scripts
cd ..
node scripts/build-frontend.mjs
node --test tests/frontend.test.mjs
.\scripts\build.ps1 -RebuildFrontend
```

npm 安装会联网获取开发依赖；最终 EXE 无需联网下载这些包。已有全局 `tsc` 时构建脚本也可以使用它，发布者必须记录实际 `tsc --version`。脚本将 TypeScript 编译为 ES 模块，并把 CSS/SVG 复制到 `frontend/dist/`。`frontend/embed.go` 通过 go:embed 将这些资源装入 EXE。

## 公共 WebView2 Loader 与单 EXE 实验回退

生产发布推荐使用 Microsoft 官方 SDK 的 **x64 WebView2Loader.dll**，或改为官方静态 Loader 并重新验收。当前源码优先加载 EXE 同目录中的公共 Loader；未发现时才扫描已安装的 WebView2 Runtime 并调用内部 `CreateWebViewEnvironmentWithOptionsInternal`。

**内部入口并非稳定的应用兼容承诺，可能随 Runtime 改变。本次没有验证它在 Windows 上能否正常初始化。** 本包没有因为 Runtime 的 DLL 存在就宣称原生窗口已经通过验收。

取得并核实官方 Microsoft.Web.WebView2 SDK 中的 x64 Loader 后：

```powershell
.\scripts\build.ps1 -WebView2LoaderPath 'C:\VerifiedSDK\runtimes\win-x64\native\WebView2Loader.dll'
```

参数路径是示例，按实际官方 SDK 解压位置填写。脚本仅复制本地文件，不下载、不认证来源、不替你完成许可审核；应保留 SDK 版本、原始来源和 SHA256。公共 Loader 是运行辅助 DLL，不是 Runtime。使用它后分发将是 EXE + Loader DLL，而不是单 EXE；这是稳定性优先于文件数的取舍。

公共 Loader / 内部入口官方说明：
https://learn.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/webview2-idl

## 测试

```powershell
.\scripts\test.ps1 -Frontend
# 具备 Go 支持的主机 C 工具链时，可追加 -Race：
.\scripts\test.ps1 -Frontend -Race
.\scripts\acceptance.ps1
.\scripts\soak.ps1 -Seconds 3600
```

`test.ps1` 执行当前主机的 go vet、Go 测试和可选前端测试。`acceptance.ps1` 仅记录机器/文件元数据并打开真实窗口，复制人工验收表，**不自动填 PASS**。`soak.ps1` 是至少一小时的后端只读采样检查，不代替完整 GUI 压力测试。各脚本不自动运行管理工具、修改开机启动或发送联网探测。

实际 UI 渲染测试可选安装 Python Playwright（仅开发依赖）。本包的离线渲染脚本适用于受管理的 Chromium 禁止 URL 导航时，使用 `about:blank` 文档和真实本地 HTTP 测试桥；不会修改浏览器管理策略：

```powershell
# 先在另一个终端启动带控制台输出的开发构建，并保留其会话地址：
go run ./cmd/jiahao-toolbox --serve --data-dir ./test-output/ui-profile
# 第二个终端，将命令输出的完整 /start/... 地址放入环境变量：
$env:JIAHAO_TEST_URL='http://127.0.0.1:实际端口/start/实际随机会话'
$env:JIAHAO_CHROMIUM='C:\实际路径\chrome.exe'
python tests/ui_matrix.py
```

UI 脚本中的 Linux 专属工具禁用断言是针对本次 Linux 测试环境设计，不是 Windows 自动验收套件。在真实 Windows/WebView2 中，使用 `docs/WINDOWS_ACCEPTANCE.md` 对所有启用入口做人工核实；不要删除禁用断言后假称整个脚本已验证原生宿主。

## Linux 交叉构建与原始评估构建复现

```sh
./scripts/build.sh
# 仅为了复现当前旧工具链评估包；不是安全认可：
ALLOW_LEGACY_TOOLCHAIN=1 ./scripts/build.sh
```

Windows 下对应 `-AllowLegacyToolchain`。这些选项不更改 EXE 的 Beta 身份，不解除发布阻断项。构建时间、Go 补丁、资源内容或源码任一变更都会改变哈希；要做位级复现，必须使用同一工具链、同一源文件、相同构建时间/版本 ldflags 和 `-trimpath -buildvcs=false`。仅固定版本号不足以宣称可重复二进制。

## 项目结构

- `cmd/jiahao-toolbox/`：启动参数与主程序；`app_windows_amd64.syso` 为嵌入图标和 PMv2 manifest。
- `cmd/resourcegen/`：可离线运行的纯 Go AMD64 COFF 资源生成器。
- `internal/app/`：会话、本地 HTTP 桥、生命周期、请求验证。
- `internal/monitor/`：统一采样、环形缓冲区、Windows API/PDH/DXGI、Linux 开发读数。
- `internal/native/`：Win32、COM、WebView2 宿主与 Runtime 查找。
- `internal/hardware/`：异步 CIM、超时、取消、逐类信息与隐私过滤。
- `internal/diagnostics/`、`systemtools/`、`config/`、`logging/`：对应独立业务模块。
- `frontend/src/`：组件、页面路由、图表、服务、样式与原始 SVG；`dist/`：随包编译结果。
- `resources/`、`scripts/`、`tests/`、`docs/`：资源、脚本、测试与记录。

没有第三方 Go 模块，所以没有虚构的 go.sum；没有 Electron、CDN、遥测服务、Web 字体或 Python 运行依赖。
