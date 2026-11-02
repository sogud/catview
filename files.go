package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
)

type documentKind uint8

const (
	kindUnknown documentKind = iota
	kindText
	kindMarkdown
	kindHTML
	kindOffice
	kindImage
	kindPDF
	kindEPUB
)

const (
	maxTextBytes  = 12 << 20
	maxImageBytes = 36 << 20
)

type fileEntry struct {
	name string
	dir  bool
}

type document struct {
	path               string
	kind               documentKind
	source             string
	digest             [32]byte
	mode               fs.FileMode
	code               *editor.State
	picture            ui.ImageSource
	md                 markdown.Options
	htmlRich           bool
	officeHTML         string
	officePresentation bool
	officeLegacy       bool
	book               *epubBook
	editing            bool
}

func (d *document) canEdit() bool {
	return d != nil && (d.kind == kindText || d.kind == kindMarkdown || d.kind == kindHTML)
}

func (d *document) label() string {
	switch d.kind {
	case kindText:
		if d.code != nil {
			return fmt.Sprintf("%d 行 · 源码 / 文本", d.code.Buffer.LineCount())
		}
	case kindMarkdown:
		if d.htmlRich {
			return "Markdown · HTML 阅读"
		}
		return "Markdown · 原生阅读"
	case kindHTML:
		return "HTML · 安全预览"
	case kindOffice:
		if d.officePresentation {
			return "PowerPoint · 阅读"
		}
		return "Word · 阅读"
	case kindImage:
		return "图片 · 原生预览"
	case kindPDF:
		return "PDF · 系统内嵌预览"
	case kindEPUB:
		return "EPUB · 内嵌阅读"
	}
	return "文件"
}

func (a *application) setRoot(chosen string, isDir bool) {
	if a.doc != nil && a.doc.code != nil && a.doc.code.Buffer.Modified() {
		a.status = "当前文件有未保存修改，请先保存或取消编辑"
		return
	}
	path, err := filepath.Abs(chosen)
	if err != nil {
		a.status = err.Error()
		return
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		a.status = "路径不可访问：" + err.Error()
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() != isDir {
		a.status = "所选文件或目录无法打开"
		return
	}
	a.closeEPUB()
	a.closeHTMLPreview()
	if a.pdf != nil {
		a.pdf.Destroy()
		a.pdf = nil
	}
	a.root = path
	a.rootIsFile = !isDir
	a.selected = ""
	a.doc = nil
	a.loadSerial++
	a.treeVersion++
	a.win.SetTitle(filepath.Base(path))
	a.win.SetRepresentedFile("")
	a.folders = make(map[string][]fileEntry)
	a.folderErrors = make(map[string]string)
	a.folderLoading = make(map[string]bool)
	a.knownFolders = make(map[string]bool)
	a.fileOutline.Open.Clear()
	a.fileRow = -1
	if isDir {
		a.knownFolders[path] = true
		a.fileOutline.Open.Add(path)
	} else {
		a.fileRow = 0
	}
	a.status = "已打开 " + filepath.Base(path)
	if isDir {
		a.rememberFolder(path)
		a.loadDirectory(path)
	} else {
		a.openDocument(path)
	}
	a.installMenu()
}

func (a *application) loadDirectory(path string) {
	if a.folderLoading[path] {
		return
	}
	a.folderLoading[path] = true
	generation := a.treeVersion
	win := a.win
	go func() {
		rows, err := directoryEntries(path)
		win.Update(func() {

			if generation != a.treeVersion {
				return
			}
			delete(a.folderLoading, path)
			if err != nil {
				a.folderErrors[path] = err.Error()
				a.status = "无法读取目录：" + filepath.Base(path)
			} else {
				a.folders[path] = rows
				for _, entry := range rows {
					if entry.dir && entry.name != "" {
						a.knownFolders[filepath.Join(path, entry.name)] = true
					}
				}
			}
		})
	}()
}

func directoryEntries(path string) ([]fileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	rows := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".DS_Store" || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		rows = append(rows, fileEntry{name: entry.Name(), dir: entry.IsDir()})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].dir != rows[j].dir {
			return rows[i].dir
		}
		return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name)
	})
	return rows, nil
}

func (a *application) inScope(path string) bool {
	if a.root == "" {
		return false
	}
	root := a.root
	if a.rootIsFile {
		root = filepath.Dir(root)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (a *application) openDocument(path string) {
	if a.doc != nil && a.doc.code != nil && a.doc.code.Buffer.Modified() {
		a.restoreSelectedFileRow()
		a.status = "有未保存改动，请先保存或取消编辑"
		return
	}
	if !a.inScope(path) {
		a.restoreSelectedFileRow()
		a.status = "拒绝打开所选目录之外的文件"
		return
	}
	a.status = "正在读取：" + filepath.Base(path)
	a.loadSerial++
	serial := a.loadSerial
	win := a.win
	go func() {
		d, err := readDocument(path)
		win.Update(func() {
			if serial != a.loadSerial {
				return
			}
			if err != nil {
				log.Printf("CatView: failed to read %s: %v", filepath.Base(path), err)
				a.restoreSelectedFileRow()
				a.status = "打开失败：" + err.Error()
				return
			}
			a.closeEPUB()
			a.closeHTMLPreview()
			if d.kind != kindPDF && a.pdf != nil {
				a.pdf.Destroy()
				a.pdf = nil
			}
			a.doc = d
			a.selected = path
			if d.kind == kindMarkdown {
				d.md.LoadImage = a.localImageLoader(path)
			}
			a.status = "已打开 " + filepath.Base(path)
			a.win.SetTitle(filepath.Base(path))
			a.win.SetRepresentedFile(path)
			if d.kind == kindPDF {
				a.openPDF(path)
			} else if d.kind == kindEPUB {
				a.openEPUB(d)
			} else if d.kind == kindHTML || d.kind == kindOffice || (d.kind == kindMarkdown && d.htmlRich) {
				a.openHTMLPreview(d)
			}
			a.installMenu()
		})
	}()
}

func readDocument(path string) (*document, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("暂不支持符号链接或非普通文件")
	}
	d := &document{path: path, mode: info.Mode()}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown", ".mdown":
		d.kind = kindMarkdown
	case ".htm", ".html":
		d.kind = kindHTML
	case ".docx", ".pptx", ".doc", ".ppt":
		d.kind = kindOffice
		if info.Size() > 50<<20 {
			return nil, errors.New("Office 文件超过 50 MB，暂不加载")
		}
		content, presentation, legacy, err := readOfficePreview(path)
		if err != nil {
			return nil, err
		}
		d.officeHTML = content
		d.officePresentation = presentation
		d.officeLegacy = legacy
		return d, nil
	case ".pdf":
		d.kind = kindPDF
		return d, nil // Let the system webview stream the PDF from disk.
	case ".epub":
		d.kind = kindEPUB
		if info.Size() > 100<<20 {
			return nil, errors.New("EPUB 超过 100 MB")
		}
		book, err := parseEPUB(path)
		if err != nil {
			return nil, err
		}
		d.book = book
		return d, nil
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		d.kind = kindImage
		if info.Size() > maxImageBytes {
			return nil, errors.New("图片超过 36 MB，暂不加载")
		}
	default:
		d.kind = kindText
		if info.Size() > maxTextBytes {
			return nil, errors.New("文本超过 12 MB，暂不加载")
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if d.kind == kindImage {
		d.picture, err = ui.DecodeBitmap(data)
		return d, err
	}
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) != -1 {
		return nil, errors.New("此文件不是 UTF-8 文本，暂不支持预览")
	}
	d.source = string(data)
	if d.kind == kindMarkdown {
		d.htmlRich = requiresHTMLRendering(d.source)
	}
	d.digest = sha256.Sum256(data)
	buf := editor.NewBuffer(d.source)
	buf.SetLanguage(editor.LanguageFor(path))
	d.code = &editor.State{
		Buffer: buf, ReadOnly: true, LineNumbers: editor.LineNumbersOff,
		NoFolding: true, NoIndentGuides: true, NoActiveLine: true,
		SoftWrap: d.kind == kindMarkdown || ext == ".txt",
	}
	return d, nil
}

// Markdown image URLs are treated as untrusted input. Only images inside the
// selected folder can be opened; URLs, symlinks and active SVG are rejected.
func (a *application) localImageLoader(markdownPath string) func(string) (ui.ImageSource, error) {
	dir := filepath.Dir(markdownPath)
	return func(ref string) (ui.ImageSource, error) {
		if ref == "" || filepath.IsAbs(ref) || strings.Contains(ref, ":") || strings.HasPrefix(ref, "//") {
			return nil, os.ErrPermission
		}
		asset := filepath.Join(dir, filepath.FromSlash(ref))
		if !a.inScope(asset) {
			return nil, os.ErrPermission
		}
		info, err := os.Lstat(asset)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxImageBytes {
			return nil, os.ErrPermission
		}
		switch strings.ToLower(filepath.Ext(asset)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
			data, err := os.ReadFile(asset)
			if err != nil {
				return nil, err
			}
			return ui.DecodeBitmap(data)
		}
		return nil, os.ErrPermission
	}
}

func (a *application) openPDF(path string) {
	if a.pdf == nil {
		web, err := a.win.NewWebView(mygo.WebViewOptions{
			Page:            mygo.PageOptions{Untrusted: true},
			BackgroundColor: "light-dark(#FBFAF7,#202225)",
		})
		if err != nil {
			a.status = "PDF 视图创建失败：" + err.Error()
			return
		}
		a.pdf = web
	}
	// On macOS, LoadFile grants read access to the file's own directory.
	// The web page is untrusted: no Go bridge is exposed to its content.
	if err := a.pdf.Page().LoadFile(path); err != nil {
		a.status = "PDF 打开失败：" + err.Error()
	}
	a.win.Invalidate()
}

func (a *application) cancelEditing() {
	d := a.doc
	if d == nil || !d.editing {
		return
	}
	if d.code.Buffer.Modified() {
		win := a.win
		go func() {
			result, err := mygo.Dialog.Message(mygo.MessageOptions{
				Parent: win, Type: mygo.MessageWarning,
				Message:       "放弃未保存的改动？",
				Detail:        "这些改动尚未写入磁盘。",
				Buttons:       []string{"继续编辑", "放弃改动"},
				DefaultButton: 0, CancelButton: 0,
			})
			if err != nil || result.Button != 1 {
				return
			}
			win.Update(func() {
				if a.doc == d {
					d.code.Buffer.Reset(d.source)
					d.editing = false
					a.status = "已放弃改动"
				}
			})
		}()
		return
	}
	d.editing = false
	a.status = "阅读模式"
}

func (a *application) saveDocument() {
	d := a.doc
	if d == nil || !d.canEdit() || !d.editing || d.code == nil || !d.code.Buffer.Modified() {
		return
	}
	text := d.code.Buffer.String()
	version := d.code.Buffer.Version()
	oldHash := d.digest
	mode := d.mode
	path := d.path
	a.status = "正在保存…"
	win := a.win
	go func() {
		newHash, err := safeSaveText(path, oldHash, mode, text)
		win.Update(func() {
			if a.doc != d {
				return
			}
			if err != nil {
				a.status = "保存失败：" + err.Error()
				return
			}
			d.digest = newHash
			d.source = text
			if d.kind == kindMarkdown {
				d.htmlRich = requiresHTMLRendering(text)
			}
			if d.kind == kindMarkdown || d.kind == kindHTML {
				if d.kind == kindHTML || d.htmlRich {
					a.openHTMLPreview(d)
				} else {
					a.closeHTMLPreview()
				}
			}
			if d.code.Buffer.Version() == version {
				d.code.Buffer.MarkSaved()
				d.editing = false
			}
			a.status = "已保存 " + filepath.Base(path)
		})
	}()
}

func safeSaveText(path string, expected [32]byte, mode fs.FileMode, text string) ([32]byte, error) {
	var zero [32]byte
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return zero, errors.New("目标已被移动、替换或不是普通文件")
	}
	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	before := sha256.New()
	_, err = io.Copy(before, file)
	closeErr := file.Close()
	if err != nil {
		return zero, err
	}
	if closeErr != nil {
		return zero, closeErr
	}
	var current [32]byte
	copy(current[:], before.Sum(nil))
	if current != expected {
		return zero, errors.New("文件已被外部程序修改；请重新打开或另存")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".catview-save-*")
	if err != nil {
		return zero, err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(mode.Perm()); err != nil {
		tmp.Close()
		return zero, err
	}
	data := []byte(text)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return zero, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return zero, err
	}
	if err := tmp.Close(); err != nil {
		return zero, err
	}
	// Rename uses the original directory, keeping a complete version of
	// the file in place even if the application closes while writing.
	if err := os.Rename(tmp.Name(), path); err != nil {
		return zero, err
	}
	return sha256.Sum256(data), nil
}
