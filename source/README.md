# 嘉豪工具箱 v4.4.0

在 v4.3 工程上增加简体中文 Windows x64 安装器、绿色 Portable ZIP、GitHub Release 版本检查和手动官方更新入口。安装默认当前用户目录 `%LOCALAPPDATA%\Programs\JiahaoToolbox`；可切换全用户安装。卸载默认保留配置、日志和测速历史。

软件保留现有 Go/WebView2 架构、深浅主题、高 DPI、硬件监控与平滑曲线、离线拼音搜索、系统工具、诊断、启动动画、网络/磁盘测速和经典 CMD。

设置中的“软件更新”只查询指定仓库 `stone2013/JiahaoToolbox`；启动检查和测试版检查默认关闭。当前没有可信更新公钥或 Authenticode 证书，因此自动下载、执行和安装被禁用；发现版本时可打开固定官方 Releases 页面手动检查。仅有 Release 同包 SHA256 不视为来源认证。安装程序和 EXE 当前未签名。

Windows 10/11 x64 需要 Microsoft Edge WebView2 Runtime。安装器检测运行时；缺失时提示并可由用户打开微软官方页面手动安装。不会捆绑或静默安装 Runtime。

本地构建见 `BUILD.md`，安装/发布操作见 `RELEASE_GUIDE.md`。GitHub Actions 仅在 PR、版本标签或手动执行时构建，不会因普通提交创建正式 Release。仓库目前已在本次任务中获用户授权公开源码与 Actions 工作流；不会自动发布 Release。
