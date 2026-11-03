# CatView

**Open and read.** A lightweight, local-first file reader for macOS, built with [MyGo](https://github.com/egoist/mygo).

像 Finder 一样浏览文件，像 Preview 一样专注阅读。单侧栏、系统风格、无需导入或建立资料库。

## 功能

- **浏览**：单击文件夹名称或图标展开/折叠；系统文件图标与 Material 图标可在设置中切换；最近 8 个文件夹与自动恢复。
- **标签页**：打开多个文件、切换与关闭；同一文件不重复打开，未保存改动在关闭前确认。
- **Markdown / HTML**：Markdown 排版、表格、图片；常用 HTML 经过安全过滤后预览。
- **代码与文本**：Go、TS、JS、Python、JSON、YAML 等语法高亮；默认只读，需要明确切换才可编辑。
- **PDF / EPUB / 图片**：内嵌阅读；EPUB 支持章节导航、字号与阅读进度保存。
- **Office**：DOCX/PPTX 基础图文预览，DOC 使用 macOS 系统转换，旧版 PPT 仅预览首页。
- **外观**：浅色 / 深色 / 跟随系统、石墨 / 雾蓝 / 苔绿强调色、字号、行距、正文宽度。

默认离线阅读，无默认遥测。只有主动选择「翻译 → 翻译文档」并确认服务后，才发送当前 Markdown / TXT 正文；原文件不变。`CatView → 检查更新…` 仅在主动操作时查询 GitHub 发布版本，有新版时可打开官方下载页，不自动安装。

## 下载

前往 [Releases](https://github.com/sogud/catview/releases) 下载 **macOS Apple Silicon (arm64)** 预览版，解压后将 `CatView.app` 拖入「应用程序」。

> **Alpha 提示**：当前构建仅做本地临时签名，**尚未经过 Apple 公证**，首次打开可能被 macOS Gatekeeper 拦截。请通过「系统设置 → 隐私与安全性」确认来源后再决定是否打开。暂不提供 Intel Mac 的预编译安装包。

### 一行安装（推荐）

在「终端」里运行（Apple Silicon，macOS 12+）：

```sh
curl -fsSL https://raw.githubusercontent.com/sogud/catview/main/install.sh | sh
```

脚本会下载最新版本（包括预览版），校验 SHA-256，把 `CatView.app` 装到「应用程序」（没有写入权限时装到 `~/Applications`）。用这种方式安装不会弹出 Gatekeeper 拦截。指定版本：`curl -fsSL … | VERSION=v0.1.0-alpha.1 sh`；卸载：`curl -fsSL … | sh -s -- --uninstall`（不删设置和数据）。安装前可以先看看[脚本内容](install.sh)。

<details>
<summary>用浏览器下载后提示「无法验证开发者」</summary>

方法一：在终端运行

```sh
xattr -dr com.apple.quarantine /Applications/CatView.app
```

方法二：先双击打开一次，然后进入 **系统设置 → 隐私与安全性**，在下方找到「已阻止打开 CatView」，点 **仍要打开**，再输入密码确认。

</details>

## 快捷键

| 操作 | 快捷键 |
| --- | --- |
| 打开文件 | ⌘O |
| 打开文件夹 | ⇧⌘O |
| 显示 / 隐藏侧栏 | ⌘B |
| 设置 | ⌘, |
| 关闭当前标签页 | ⌘W |
| 关闭窗口 | ⇧⌘W |
| 查找（支持的源码视图） | ⌘F |
| 保存（显式编辑后） | ⌘S |

可通过 macOS「文件」菜单切换最近文件夹。正文与文件名不会额外占用标题栏空间。

## 翻译与界面语言（本地开发版）

设置中可选 18 种界面语言或跟随系统；核心菜单与设置使用 CatView 自有字典；保存/关闭/更新提示已覆盖英文、简体和繁体中文，其余语言缺项回退英文。部分格式阅读错误仍沿用旧提示，未完成全部语言校对与 RTL 布局验收。

翻译复用 `translate-kit`：仅 Markdown 与 `.txt` / `.text`（小于 1 MiB），原文/译文/双语切换，按标签页隔离、可取消，进入编辑或关闭标签即取消。PDF、EPUB、Office 不做全文翻译。译文仅保留在内存，不覆盖原文件。

在设置填写服务、地址、模型、目标语言与密钥后保存。密钥框遮罩，文件保存在应用数据目录 `translation.json`，原子写入且权限 `0600`；这是本地明文文件，不是加密保险库，不使用 macOS Keychain。Google/有道网页接口非官方。未进行真实服务、密钥与端到端翻译验收。

## 从源码构建

本地开发暂用 `github.com/sogud/translate-kit v0.0.0`，通过 `replace` 指向同级 `../translate-kit` 源码。共享包尚无已发布版本，单独克隆 CatView 还不能完成构建。现有 GitHub Alpha 下载不包含本轮源码新增功能。

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
- 切换标签页会重建 PDF/HTML/Office 预览，暂不保留这些格式的滚动位置；EPUB 使用已记录的阅读进度。
- 文件夹行点击、标签页交互、检查更新菜单与设置窗口仍待辅助功能操作验收。

## 开源协议

[MIT](LICENSE)。文件类型图标来自 [Material Icon Theme](https://github.com/material-extensions/vscode-material-icon-theme)（MIT），具体许可见 [图标声明](assets/filetype-material-icon-theme-LICENSE.txt)。依赖库许可见 [第三方声明](THIRD_PARTY_LICENSES.txt)。

项目名 CatView 暂未完成商标可用性核查，预发布不构成商标独占声明。
