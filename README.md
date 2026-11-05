# CatView

**Open and read.** 一个轻量的 macOS 本地文件阅读器，基于 [MyGo](https://github.com/egoist/mygo) 构建。

打开文件或文件夹即可阅读。单列文件树、多标签页、原生窗口，无需导入或建立资料库。

[下载 Alpha 6](https://github.com/sogud/catview/releases/tag/v0.1.0-alpha.6) · [更新记录](CHANGELOG.md) · [反馈问题](https://github.com/sogud/catview/issues)

## 安装

当前预编译版本支持 **macOS 12+、Apple Silicon（arm64）**，暂不提供 Intel Mac 安装包。

1. 从 [Releases](https://github.com/sogud/catview/releases) 下载 `CatView-…-macos-arm64.zip`。
2. 解压，将 `CatView.app` 拖入「应用程序」。
3. 首次打开若被系统阻止，可在确认来源后进入「系统设置 → 隐私与安全性」，选择「仍要打开」。

当前为 **Alpha**：应用仅做临时签名，尚无正式 Developer ID 签名与 Apple 公证。更新包的 Ed25519 签名不等同于 Apple 公证。手动安装请选择 ZIP；Release 中的 `tar.gz` 和 JSON 文件供自动更新使用。

<details>
<summary>通过终端安装或卸载</summary>

先查看 [安装脚本](install.sh)，确认后运行：

```sh
curl -fsSL https://raw.githubusercontent.com/sogud/catview/main/install.sh | sh
```

脚本下载最新版本（含预览版）并核对发布的 SHA-256，安装到 `/Applications`；没有写入权限时使用 `~/Applications`。它会移除应用的 quarantine 标记，请只从可信来源运行。

指定版本：

```sh
curl -fsSL https://raw.githubusercontent.com/sogud/catview/main/install.sh | VERSION=v0.1.0-alpha.6 sh
```

卸载应用，保留设置和数据：

```sh
curl -fsSL https://raw.githubusercontent.com/sogud/catview/main/install.sh | sh -s -- --uninstall
```

</details>

## 开始使用

- 初始页可打开文件或文件夹，也可通过「文件」菜单返回最近打开的目录。
- 单击文件夹展开或折叠，单击文件在新标签打开；同一文件会复用已有标签。
- 文件树右键可新建文件/文件夹、重命名、移到废纸篓，或在 Finder 中定位。同名项目不会被覆盖，删除前需确认。
- 文本默认只读。在「文件 → 编辑文件」中显式开启编辑；保存时检测外部修改，关闭时提示未保存内容。

| 操作 | 快捷键 |
| --- | --- |
| 打开文件 / 文件夹 | ⌘O / ⇧⌘O |
| 显示或隐藏侧栏 | ⌘B |
| 设置 | ⌘, |
| 关闭标签 / 窗口 | ⌘W / ⇧⌘W |
| 查找（源码视图） | ⌘F |
| 保存（编辑模式） | ⌘S |

## 支持的内容

以下以 Alpha 6 为准；系统解码能力与未逐项验证的格式见下方限制。

| 类型 | 当前能力 |
| --- | --- |
| Markdown / HTML | 表格、图片、代码高亮与锚点；Markdown 的本地文件链接在标签页打开，范围限于已授权文件夹 |
| Mermaid | 离线流程图、时序图等；Codex 风格的蓝色明暗配色、圆角节点和细线；语法错误时保留源码 |
| 代码 / 文本 | Go、TS、JS、Python、JSON、YAML 等语法高亮与按需编辑；小型 JSON、XML 和 XML plist 可格式化阅读，编辑仍使用原文 |
| CSV / TSV | 原生表格、可调列宽、单元格复制；保留前导零和长整数，可切换源码 |
| 图片 / SVG | PNG、JPEG、WebP、BMP、HEIC/HEIF、AVIF、ICO/ICNS；适应窗口或缩放。GIF 可播放动画，TIFF 可逐页阅读；SVG 支持静态图形，拒绝脚本、动画和嵌入资源 |
| 专业图片 | PSD、TGA、EXR、JPEG 2000、JPEG XL、DDS 与 DNG/CR2/CR3/NEF/NRW/ARW/SR2/RAF/ORF/RW2/PEF/SRW 路由至 ImageIO；仅系统能解码时显示合成图或预览图，不读取图层或进行 RAW 编辑 |
| 视频 / 音频 | MP4、M4V、MOV、M4A、MP3、WAV、AIFF、AAC、CAF、FLAC、AC3/EAC3 交给 macOS AVKit；打开不自动播放，离开标签停止，再次进入从头暂停 |
| PDF | 窗口内使用系统 WebKit PDF 阅读 |
| EPUB | 可重排正文、章节导航、字号和进度恢复；不支持 DRM 或固定版式 |
| Office | DOCX/PPTX 基础图文预览；DOC 使用系统转换；旧版 PPT 仅预览首页 |
| 系统文档 | XLSX、XLS、RTF、ODT、Pages/Numbers/Keynote 交给 Quick Look，效果取决于系统和已安装的预览扩展；不支持目录式文档包 |
| 邮件 / 日历 | EML 正文、邮件头和附件名称；ICS 事件、时区与全天日期；只读，可查看 UTF-8 源码，不访问账户、不导入事件 |
| 压缩包 | ZIP、TAR、TAR.GZ/TGZ、TAR.BZ2/TBZ2 只读目录与大小；7z、RAR/CBR 借用系统 bsdtar 列出目录，不解压或打开包内文件 |
| CBZ | 按文件名自然排序逐页阅读静态图片，只在内存读取当前页，不解压到磁盘 |

已实机确认 CSV/TSV、SVG、HEIC、TIFF 首图、AVIF/ICNS、JSON、ZIP、基础 XLSX/RTF/ODT 和 H.264 MP4；MP3 的播放/暂停与 WAV 初始状态也已检查。集中验收另确认了 EML 正文/附件名称、ICS 事件和源码切换、TAR.GZ/7z 目录、CBZ 自然排序/翻页、TIFF 第二页及 GIF 动画循环；损坏的 ICS/7z 显示可见错误。ICO、RAR、TAR.BZ2、专业图片和新增音频编码尚未逐一实机验收。音视频容器不代表所有编码均可播放，Quick Look、RAW 相机型号和压缩方法取决于系统。

「显示 → 系统预览」可为 Office/PDF 切换系统原版式预览，也可尝试 DOCM/PPTM/XLSM/XLSB/ODP/ODS；没有相应系统扩展或文档不兼容时可能仅显示文件图标，不保证能阅读这些格式。本轮 DOCX 样本在 Quick Look 中只显示图标，原有基础预览保留。图片缩放和「显示源码」也位于该菜单；⌘0 适应窗口，⌘1 显示预览位图的 100%，⌘+/⌘− 调整大小。多页 TIFF 和 CBZ 使用「阅读」菜单或 ⌥←/⌥→ 翻页。读取失败会显示原因，并提供重试、默认应用和 Finder 入口。

## 外观与更新

默认 macOS 主题采用白色阅读区和系统侧栏材质。可切换暖白、GitHub、Nord、森林、薰衣草、砂岩，共 7 套明暗主题；也可设置强调色、字号、行距与正文宽度。

Alpha 5 起默认后台检查并下载更新，验证签名后提示「安装并重启」，点击后才安装；未保存的编辑会阻止安装。设置 → 更新可分别关闭自动检查、自动下载；「CatView → 检查更新…」保留手动入口。自动检查约每 6 小时一次，网络或限流失败会稍后重试。

**Alpha 4 及更早版本需要先手动安装 Alpha 5 一次。** 后续更新需应用所在文件夹可写；已下载的更新可离线安装。

## 本地数据与翻译

文档在本机阅读，不默认发送正文，无默认遥测。自动更新会访问公开 GitHub 发布信息。

翻译仅支持小于 1 MiB 的 Markdown / TXT。主动选择翻译并确认服务后，才向所选服务发送当前正文；支持原文、译文和双语查看，不覆盖文件。服务配置与密钥以本地明文保存，仅当前用户可读写，未使用 Keychain；Google/有道网页接口非官方。真实翻译服务尚未完成端到端验收。

界面可选 18 种语言或跟随系统，缺失译文回退英文；并非所有语言和 RTL 布局都已校对。

## 从源码构建

在 macOS 上安装符合 [go.mod](go.mod) 要求的 Go，进入仓库后运行：

```sh
go mod download
go run github.com/egoist/mygo/cmd/mygo@v0.7.3 build -skip-dmg
```

Apple Silicon 产物位于 `build/darwin-arm64/CatView.app`。依赖由 Go 下载，无需同级源码目录；其他平台尚未完整验证。

普通构建不需要发布私钥。维护者通过 `MYGO_UPDATER_PRIVATE_KEY` 提供原有 Ed25519 密钥，才会生成签名更新包与版本清单；须与 ZIP 一起上传到对应 Release。私钥不得提交或随应用分发，后续更新须使用同一密钥。

## 已知限制

- Office 仅作基础预览，复杂排版、字体、母版和动画可能失真；EPUB 不支持全部出版社样式。
- 文本（含 Markdown/HTML/CSV）限 12 MiB；表格最多显示 20,000 行、128 列或 200,000 单元格。JSON/XML 自动格式化限 2 MiB，超过时仍可看源码。
- 图片文件限 36 MiB、源图限 1 亿像素，预览长边缩至最多 4096 像素。GIF 最多 300 帧、每边 4096 像素，画布像素乘帧数不超过 64 Mi；TIFF 最多 512 页。AVIF 等解码能力取决于 macOS 版本。
- SVG 限 4 MiB。ZIP 和系统文档限 50 MiB，ZIP 最多 20,000 条目。CBZ 限 100 MiB、2,000 张图片，单张解压后限 36 MiB，不支持加密、活动图片和不安全路径。
- TAR 与 7z/RAR 限 100 MiB、20,000 条目；TAR 扫描解压数据限 256 MiB，7z/RAR 目录输出限 2 MiB，最多运行 8 秒。不支持加密目录、所有 RAR 变体或压缩方法；CBR 只列目录，不作为漫画翻页。
- EML 限 12 MiB、200 个 MIME 部件和 16 层；正文单段 2 MiB、合计 4 MiB，不加载图片或打开附件。ICS 限 1 MiB、512 个组件和 32 层，仅展示 VEVENT，不展开重复事件或执行提醒；自定义时区保留原值。
- XLSX 使用系统只读预览，不提供编辑或公式计算；本机 ODS 未显示表格。MKV、AVI、WebM、OGG/Opus、MOBI/AZW/KFX、DRM 及 TAR.XZ 尚未接入。专业图片只作有限尺寸预览，不保证各相机型号、旧 macOS、HDR 或色彩工作流。
- 切换标签会重建 PDF/HTML/Office 预览，暂不保留这些格式的滚动位置。
- 部分带 HTML 的小型 Markdown 文件通过命令行直接打开时，可能停留在空阅读区。
- 当前打开的根文件夹不能在文件树中重命名或删除，可在 Finder 中操作。

## 开源协议

[MIT](LICENSE)。文件图标来自 [Material Icon Theme](https://github.com/material-extensions/vscode-material-icon-theme)，见 [图标许可](assets/filetype-material-icon-theme-LICENSE.txt)；Mermaid 与其他依赖见 [第三方声明](THIRD_PARTY_LICENSES.txt)。

CatView 名称和猫图形尚未完成商标可用性核查，预发布不构成商标独占声明。
