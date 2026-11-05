package main

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Material Icon Theme v5.40.0, MIT (Material Extensions).
// Only the icons used by CatView are embedded. The upstream license is in
// assets/filetype-material-icon-theme-LICENSE.txt.
//
//go:embed assets/filetype-*.svg
var fileIconAssets embed.FS

var fileIcons = loadFileIcons()

func loadFileIcons() map[string]*ui.SVG {
	icons := make(map[string]*ui.SVG)
	paths, err := fs.Glob(fileIconAssets, "assets/filetype-*.svg")
	if err != nil {
		return icons
	}
	for _, filename := range paths {
		data, err := fileIconAssets.ReadFile(filename)
		if err != nil {
			continue
		}
		icon, err := ui.ParseSVG(data)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(filename), "filetype-"), ".svg")
		icons[name] = icon
	}
	return icons
}

// treeIcon uses the actual Finder icon, including custom file and folder icons.
// Cache decoded Retina-sized images for this tree, not on every UI frame.
func (a *application) treeIcon(path string, isDirectory bool, expanded bool) ui.ImageSource {
	if a.fileIconStyle == 0 {
		if a.systemIcons == nil {
			a.systemIcons = make(map[string]ui.ImageSource)
		}
		icon, cached := a.systemIcons[path]
		if !cached {
			if data, err := mygo.Shell.FileIcon(path, 34); err == nil {
				icon, _ = ui.DecodeImage(data)
			}
			a.systemIcons[path] = icon
		}
		if icon != nil {
			return icon
		}
	}
	return iconForFile(path, isDirectory, expanded)
}

func iconForFile(path string, isDirectory bool, expanded bool) *ui.SVG {
	name := fileIconName(path, isDirectory, expanded)
	if icon := fileIcons[name]; icon != nil {
		return icon
	}
	return fileIcons["file"]
}

func fileIconName(path string, isDirectory bool, expanded bool) string {
	if isDirectory {
		if expanded {
			return "folder-open"
		}
		return "folder"
	}

	name := strings.ToLower(filepath.Base(path))
	switch name {
	case "readme", "readme.md", "readme.markdown":
		return "readme"
	case "license", "license.md", "license.txt", "copying":
		return "license"
	case "go.mod", "go.sum", "go.work":
		return "go-mod"
	case "dockerfile", ".dockerignore", "compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml":
		return "docker"
	case ".gitignore", ".gitattributes", ".gitmodules", ".gitconfig":
		return "git"
	case ".env", ".env.local", ".env.production", ".env.development":
		return "lock"
	case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "cargo.lock", "gemfile.lock":
		return "lock"
	}

	if strings.HasSuffix(name, ".d.ts") {
		return "typescript-def"
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mdx":
		return "markdown"
	case ".go":
		return "go"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx", ".jsx":
		return "react"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".vue":
		return "vue"
	case ".py", ".pyi":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".php":
		return "php"
	case ".rb", ".rake":
		return "ruby"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "css"
	case ".json", ".jsonc", ".jsonl", ".ndjson":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml", ".plist":
		return "xml"
	case ".toml", ".ini", ".cfg", ".conf":
		return "toml"
	case ".pdf":
		return "pdf"
	case ".epub":
		return "epub"
	case ".doc", ".docx", ".odt", ".rtf":
		return "word"
	case ".ppt", ".pptx", ".odp", ".key":
		return "powerpoint"
	case ".csv", ".tsv", ".xls", ".xlsx", ".ods":
		return "table"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".heic", ".heif", ".avif", ".tif", ".tiff", ".ico", ".icns", ".psd", ".tga", ".exr", ".jp2", ".j2k", ".jxl", ".dds", ".dng", ".cr2", ".cr3", ".nef", ".nrw", ".arw", ".sr2", ".raf", ".orf", ".rw2", ".pef", ".srw":
		return "image"
	case ".svg":
		return "svg"
	case ".mp3", ".wav", ".flac", ".m4a", ".aac", ".ogg", ".aif", ".aiff", ".caf", ".ac3", ".ec3":
		return "audio"
	case ".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi":
		return "video"
	case ".zip", ".tar", ".gz", ".tgz", ".7z", ".rar", ".xz", ".bz2", ".tbz2", ".cbz", ".cbr":
		return "zip"
	case ".db", ".sqlite", ".sqlite3", ".sql":
		return "database"
	case ".sh", ".zsh", ".bash", ".fish", ".ps1":
		return "console"
	case ".log":
		return "log"
	case ".lock":
		return "lock"
	}
	return "file"
}
