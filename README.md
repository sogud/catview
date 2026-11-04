# CatView

**Open and read.** A lightweight, local-first file reader for macOS, built with [MyGo](https://github.com/egoist/mygo).

像 Finder 一样浏览文件，像 Preview 一样专注阅读。单侧栏、系统风格、无需导入或建立资料库。

## 功能

- **浏览**：单击文件夹名称或图标展开/折叠；系统文件图标与 Material 图标可在设置中切换；最近 8 个文件夹与自动恢复；文件树右键支持打开、新建文件/文件夹、重命名、移到废纸篓和在 Finder 中定位。
- **标签页**：打开多个文件、切换与关闭；同一文件不重复打开，未保存改动在关闭前确认。
- **Markdown / HTML**：排版、表格、图片、HTML 锚点与中文标题跳转；代码块复用源码阅读器的语法高亮。`mermaid` 代码块离线渲染流程图、时序图等，配色跟随明暗主题，语法错误保留原始代码。
- **代码与文本**：Go、TS、JS、Python、JSON、YAML 等语法高亮；默认只读，需要明确切换才可编辑。
- **PDF / EPUB / 图片**：内嵌阅读；EPUB 支持章节导航、字号与阅读进度保存。
- **Office**：DOCX/PPTX 基础图文预览，DOC 使用 macOS 系统转换，旧版 PPT 仅预览首页。
- **外观**：默认 macOS 主题，浅色使用白色阅读区与系统半透明侧栏；可选暖白、GitHub、Nord、森林、薰衣草、砂岩，共 7 套明暗成对主题。支持跟随系统、8 种强调色、字号、行距、正文宽度；独立设置窗口使用 macOS 风格的图标分类与紧凑表单。

文档默认离线阅读，无默认遥测。只有主动选择「翻译 → 翻译文档」并确认服务后，才发送当前 Markdown / TXT 正文；原文件不变。更新检查只访问公开 GitHub 发布信息，不发送文档。

**软件更新**：启动后检查，并每 6 小时自动检查新版本；后台下载并验证签名，完成后提示「安装并重启」，点击后才安装。未保存的编辑会阻止安装；下载好的更新可在离线时安装。设置 → 更新可分别关闭自动检查、自动下载，菜单仍保留手动检查。优先读取公开 Release 订阅，减少匿名 API 限流；限流时按服务器时间等待。Alpha 4 及更早版本需先手动安装 Alpha 5 一次。

## 下载

前往 [Releases](https://github.com/sogud/catview/releases) 下载 **macOS Apple Silicon (arm64)** 预览版，解压后将 `CatView.app` 拖入「应用程序」。

> **Alpha 提示**：当前构建仅做本地临时签名，**尚未经过 Apple 公证**，首次打开可能被 macOS Gatekeeper 拦截。请通过「系统设置 → 隐私与安全性」确认来源后再决定是否打开。暂不提供 Intel Mac 的预编译安装包。

### 一行安装（推荐）

在「终端」里运行（Apple Silicon，macOS 12+）：

```sh
curl -fsSL https://raw.githubusercontent.com/sogud/catview/main/install.sh | sh
```

脚本会下载最新版本（包括预览版），校验 SHA-256，把 `CatView.app` 装到「应用程序」（没有写入权限时装到 `~/Applications`）。用这种方式安装不会弹出 Gatekeeper 拦截。指定版本：`curl -fsSL … | VERSION=v0.1.0-alpha.4 sh`；卸载：`curl -fsSL … | sh -s -- --uninstall`（不删设置和数据）。安装前可以先看看[脚本内容](install.sh)。

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

可通过 macOS「文件」菜单切换最近文件夹。窗口控件留在文件树顶部，阅读区不再有独立空白标题条；标签页与窗口控件处于同一层。

文件操作不会覆盖同名项目；删除需确认并使用系统废纸篓。重命名或删除前，需先保存/放弃相关标签的未保存编辑。当前打开的根文件夹不能直接重命名或删除，可在 Finder 中操作。

Markdown 中指向已打开文件夹内其他文件的链接会在新标签打开，已打开的文件会复用标签；支持相对路径、空格编码和 `文件.md#锚点`。同页目录链接直接滚动到目标。文件不存在或超出当前文档的授权文件夹时显示提示。

## 翻译与界面语言

设置中可选 18 种界面语言或跟随系统；核心菜单与设置使用 CatView 自有字典；保存/关闭/更新提示已覆盖英文、简体和繁体中文，其余语言缺项回退英文。部分格式阅读错误仍沿用旧提示，未完成全部语言校对与 RTL 布局验收。

翻译复用 `translate-kit`：仅 Markdown 与 `.txt` / `.text`（小于 1 MiB），原文/译文/双语切换，按标签页隔离、可取消，进入编辑或关闭标签即取消。PDF、EPUB、Office 不做全文翻译。译文仅保留在内存，不覆盖原文件。

在设置填写服务、地址、模型、目标语言与密钥后保存。密钥框遮罩，文件保存在应用数据目录 `translation.json`，原子写入且权限 `0600`；这是本地明文文件，不是加密保险库，不使用 macOS Keychain。Google/有道网页接口非官方。未进行真实服务、密钥与端到端翻译验收。

## 从源码构建

翻译依赖固定为已发布的 `github.com/sogud/translate-kit v0.1.0`，Go 会自动下载，无需同级源码目录。v0.1.0-alpha.5 新增后台检查、签名下载与点击安装重启；保留 Alpha 4 的默认 macOS 配色、系统侧栏材质及设置布局。

在 macOS 上安装 [Go](https://go.dev/)（以 `go.mod` 的版本要求为准）：

```bash
go mod download
go run github.com/egoist/mygo/cmd/mygo@v0.7.3 build -skip-dmg
```

输出：`build/darwin-arm64/CatView.app`。项目优先支持 macOS；其他平台目前未经完整验证。

维护者发布时另外通过 `MYGO_UPDATER_PRIVATE_KEY` 环境变量提供原有 Ed25519 私钥，构建工具才会生成签名的 `.tar.gz` 与 `update-darwin-arm64.json`；两者须与安装 ZIP 一起上传到对应版本的 Release。私钥不进 Git，也不随应用分发；请安全备份，后续更新须使用同一密钥。普通源码构建不需要私钥。

## 已知限制

- 当前是实验性 **Alpha**，更新包有 Ed25519 签名，应用仍未做正式 Developer ID 签名与 Apple 公证。
- Office 不是原版排版引擎；复杂版式、字体、母版、动画可能失真，旧版 PPT 仅首页。
- EPUB 不支持 DRM、固定版式或全部出版商 CSS；复杂竖排仍需更多实机验证。
- 部分带 HTML 的小型 Markdown 文件通过命令行直接打开时，可能停留在空阅读区；正在排查。
- 切换标签页会重建 PDF/HTML/Office 预览，暂不保留这些格式的滚动位置；EPUB 使用已记录的阅读进度。
- 已用临时文件实测右键新建、重命名、重名/权限错误、废纸篓取消/删除/恢复，以及标签关闭、设置切换和持久化；Markdown 图表、高亮与锚点/文件链接已实机点击验证。真实翻译服务及全部文档格式组合仍待专项验收。

## 开源协议

[MIT](LICENSE)。文件类型图标来自 [Material Icon Theme](https://github.com/material-extensions/vscode-material-icon-theme)（MIT），具体许可见 [图标声明](assets/filetype-material-icon-theme-LICENSE.txt)。依赖库许可见 [第三方声明](THIRD_PARTY_LICENSES.txt)。

项目名 CatView 暂未完成商标可用性核查，预发布不构成商标独占声明。
