# CatView

**Open and read.** A lightweight, local-first file reader for macOS, built with [MyGo](https://github.com/egoist/mygo).

像 Finder 一样浏览文件，像 Preview 一样专注阅读。单侧栏、系统风格、无需导入或建立资料库。

## 功能

- **浏览**：原生文件树、开源文件类型图标、最近 8 个文件夹与自动恢复。
- **Markdown / HTML**：Markdown 排版、表格、图片；常用 HTML 经过安全过滤后预览。
- **代码与文本**：Go、TS、JS、Python、JSON、YAML 等语法高亮；默认只读，需要明确切换才可编辑。
- **PDF / EPUB / 图片**：内嵌阅读；EPUB 支持章节导航、字号与阅读进度保存。
- **Office**：DOCX/PPTX 基础图文预览，DOC 使用 macOS 系统转换，旧版 PPT 仅预览首页。
- **外观**：浅色 / 深色 / 跟随系统、石墨 / 雾蓝 / 苔绿强调色、字号、行距、正文宽度。

不需要云账户，不上传本地文件，也没有默认遥测。

## 下载

前往 [Releases](https://github.com/sogud/catview/releases) 下载 **macOS Apple Silicon (arm64)** 预览版，解压后将 `CatView.app` 拖入「应用程序」。

> **Alpha 提示**：当前构建仅做本地临时签名，**尚未经过 Apple 公证**，首次打开可能被 macOS Gatekeeper 拦截。请通过「系统设置 → 隐私与安全性」确认来源后再决定是否打开。暂不提供 Intel Mac 的预编译安装包。

## 快捷键

| 操作 | 快捷键 |
| --- | --- |
| 打开文件 | ⌘O |
| 打开文件夹 | ⇧⌘O |
| 显示 / 隐藏侧栏 | ⌘B |
| 设置 | ⌘, |
| 查找（支持的源码视图） | ⌘F |
| 保存（显式编辑后） | ⌘S |

可通过 macOS「文件」菜单切换最近文件夹。正文与文件名不会额外占用标题栏空间。

## 从源码构建

在 macOS 上安装 [Go](https://go.dev/)（以 `go.mod` 的版本要求为准）：

```bash
go mod download
go run github.com/egoist/mygo/cmd/mygo@v0.7.3 build -skip-dmg
```

输出：`build/darwin-arm64/CatView.app`。项目优先支持 macOS；其他平台目前未经完整验证。

## 已知限制

- 当前是实验性 **Alpha**，尚无自动更新、正式 Developer ID 签名与公证。
- Office 不是原版排版引擎；复杂版式、字体、母版、动画可能失真，旧版 PPT 仅首页。
- EPUB 不支持 DRM、固定版式或全部出版商 CSS；复杂竖排仍需更多实机验证。
- 部分带 HTML 的小型 Markdown 文件通过命令行直接打开时，可能停留在空阅读区；正在排查。
- 菜单与设置窗口尚未完成完整的辅助功能操作验收。

## 开源协议

[MIT](LICENSE)。文件类型图标来自 [Material Icon Theme](https://github.com/material-extensions/vscode-material-icon-theme)（MIT），具体许可见 [图标声明](assets/filetype-material-icon-theme-LICENSE.txt)。依赖库许可见 [第三方声明](THIRD_PARTY_LICENSES.txt)。

项目名 CatView 暂未完成商标可用性核查，预发布不构成商标独占声明。
