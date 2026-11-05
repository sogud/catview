package main

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// CBZ is an image-only ZIP. Read one bounded entry directly into ImageIO;
// never extract paths or keep every decoded page in memory.
func readComicPage(ctx context.Context, filename string, page int) (*document, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	entry, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || entry.Mode()&os.ModeSymlink != 0 || !os.SameFile(entry, info) {
		return nil, errors.New("文件在读取前已被替换，请重新打开")
	}
	if info.Size() > 100<<20 {
		return nil, errors.New("CBZ 超过 100 MiB，暂不预览")
	}
	book, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, err
	}
	if len(book.File) > 20000 {
		return nil, errors.New("CBZ 条目超过 20,000 个，暂不预览")
	}
	var pages []*zip.File
	for _, file := range book.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.ReplaceAll(file.Name, "\\", "/")
		clean := path.Clean(name)
		if !file.Mode().IsRegular() || strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(path.Base(name), ".") {
			continue
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".avif", ".heic", ".heif":
		default:
			continue
		}
		if path.IsAbs(name) || clean != name || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsRune(name, 0) {
			return nil, errors.New("CBZ 包含不安全的图片路径，暂不预览")
		}
		if file.Flags&1 != 0 {
			return nil, errors.New("暂不支持加密 CBZ")
		}
		pages = append(pages, file)
		if len(pages) > 2000 {
			return nil, errors.New("CBZ 图片超过 2,000 页，暂不预览")
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("CBZ 中没有支持的静态图片")
	}
	order := collate.New(language.Und, collate.Numeric, collate.IgnoreCase)
	sort.SliceStable(pages, func(i, j int) bool { return order.CompareString(pages[i].Name, pages[j].Name) < 0 })
	if page < 0 || page >= len(pages) {
		return nil, errors.New("图片页码不存在，文件可能已改变，请重试")
	}
	selected := pages[page]
	if selected.UncompressedSize64 > maxImageBytes {
		return nil, errors.New("CBZ 当前图片超过 36 MiB，暂不预览")
	}
	r, err := selected.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(&previewReader{ctx: ctx, reader: r}, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, errors.New("CBZ 图片解压后超过预览限制")
	}
	decoded, err := decodeImagePage(data, 0)
	if err != nil {
		return nil, err
	}
	return &document{kind: kindImage, picture: decoded.picture, imagePages: len(pages)}, nil
}
