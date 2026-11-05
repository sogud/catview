# Changelog

## 0.1.0-alpha.6

- 新增 CSV/TSV 原生表格、JSON/XML 格式化阅读。
- 图片增加缩放、静态 SVG、HEIC/AVIF/图标、GIF 动画、多页 TIFF 与 CBZ 翻页。
- 接入 macOS 原生音视频播放，打开不自动播放，离开标签停止。
- 新增只读 EML 邮件与 ICS 日历预览，不访问账户或导入事件。
- ZIP/TAR 系列与系统 7z/RAR 目录浏览，不解压；增加 Quick Look 文档入口。
- 异步加载、资源限制、可见错误与重试入口。

部分专业图片、音频编码、RAR 变体及 Quick Look 文档依赖系统能力，未逐项实测；MOBI/AZW、DRM、MKV/AVI/WebM 与 TAR.XZ 尚未接入。

macOS 12+ Apple Silicon；临时签名，尚未 Apple 公证。Alpha 5 可自动发现并下载，点击安装并重启后升级。Alpha 4 及更早版本请手动安装 ZIP。

## 0.1.0-alpha.5

- 后台检查和下载更新，通过签名验证后提示「安装并重启」。
- 设置新增自动检查、自动下载开关，安装前保护未保存的编辑。
- 优先使用 GitHub Release 订阅检查版本，减少 API 限流；下载完成后可离线安装。
- Mermaid 采用 Codex 风格的蓝色明暗配色、圆角节点和细线，随阅读区宽度适配。
- 初始页增加淡色猫图标、打开文件/文件夹入口和对齐的快捷键提示。

Alpha 4 及更早版本需要手动安装此版一次。仍为 Apple Silicon Alpha，尚未经过 Apple 公证。
