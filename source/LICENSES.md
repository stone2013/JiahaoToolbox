# 许可证与第三方组件

项目源码 MIT，见 LICENSE。原创标识、图标、雪山 SVG 的原有 CC0 授权及许可证文本保留于 source/resources/licenses/。

- Go runtime / 标准库：本次 Go 1.27.2，BSD 风格许可，文本见资源目录；编译进入 EXE。
- TypeScript 5.8.3：Apache-2.0，仅构建。
- pinyin-pro 3.29.5：MIT，仅构建时转换；其许可文本保存在 resources/licenses/pinyin-pro-MIT.txt，生成的索引随程序分发。官方仓库 https://github.com/zh-lx/pinyin-pro 。
- jsdom 与 playwright-core：仅开发测试，准确版本及传递依赖见 frontend/package-lock.json，不随 EXE 运行。复现安装时保留 npm 包内许可。
- Microsoft WebView2 Runtime 与 Edge：使用本机安装组件，未打包，不受本项目 MIT 授权覆盖。
- 可选 WebView2Loader.dll：本包未附带，须遵守 Microsoft SDK 许可。
- Windows API、PDH、CIM、DXGI：沿用操作系统接口，未分发系统组件。

EXE 不含运行时 npm 包、CDN、在线拼音 API 或远程字体。源码包不附带 node_modules；使用锁文件安装构建/测试依赖。
