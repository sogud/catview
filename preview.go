package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Bound expensive decoders across tabs. Closing a tab cancels queued reads;
// a running system decoder releases its slot when the decode returns.
var previewWorkers = make(chan struct{}, 2)

func (a *application) loadDocument(target *document) {
	if target.loadCancel != nil {
		target.loadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	target.loadCancel = cancel
	target.loading, target.previewError = true, ""
	path := target.path
	imageIndex := target.imageIndex
	win := a.win
	go func() {
		select {
		case previewWorkers <- struct{}{}:
		case <-ctx.Done():
			return
		}
		d, err := readDocument(ctx, path, imageIndex)
		<-previewWorkers
		if ctx.Err() != nil {
			return
		}
		win.Update(func() {
			if ctx.Err() != nil {
				return
			}
			cancel()
			target.loadCancel = nil
			target.loading = false
			if err == nil && d.picture != nil {
				w, h := ui.ImageSize(d.picture)
				pixels := float64(w) * float64(h)
				for _, tab := range a.tabs {
					if tab != target && tab.picture != nil {
						tw, th := ui.ImageSize(tab.picture)
						pixels += float64(tw) * float64(th)
					}
				}
				if pixels > 64*1024*1024 {
					err = errors.New("已打开图片达到内存预算，请关闭部分图片标签后重试")
				}
			}
			if err != nil {
				target.previewError = err.Error()
			} else {
				// Keep the tab's identity and UI state; markdown.Options owns a
				// sync.Map and must never be copied with the whole document.
				target.kind, target.mode = d.kind, d.mode
				target.source, target.digest = d.source, d.digest
				target.code, target.formatted = d.code, d.formatted
				target.picture, target.table = d.picture, d.table
				target.imagePages, target.imageAnimated = d.imagePages, d.imageAnimated
				target.htmlRich, target.officeHTML = d.htmlRich, d.officeHTML
				target.previewHTML = d.previewHTML
				target.officePresentation, target.officeLegacy = d.officePresentation, d.officeLegacy
				target.book = d.book
				if target.kind == kindMarkdown {
					target.htmlRich = target.htmlRich || target.fragment != ""
					target.md.LoadImage = localImageLoader(target.path, target.scopeRoot)
				}
			}
			if a.doc == target {
				a.openDocumentPreview(target)
			}
			a.installMenu()
		})
	}()
}

func readPreviewBytes(ctx context.Context, path string, limit int64) ([]byte, error) {
	f, err := openPreviewFile(path, limit)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// The limited reader also catches a file growing after Stat.
	data, err := io.ReadAll(io.LimitReader(&previewReader{ctx: ctx, reader: f}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("文件超过 %d MiB 预览限制", limit>>20)
	}
	return data, nil
}

func openPreviewFile(path string, limit int64) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("只能预览普通文件")
	}
	entry, err := os.Lstat(path)
	if err != nil || entry.Mode()&os.ModeSymlink != 0 || !os.SameFile(entry, info) {
		return nil, errors.New("文件在读取前已被替换，请重新打开")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("文件超过 %d MiB 预览限制", limit>>20)
	}
	ok = true
	return f, nil
}

type previewReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *previewReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func bytesWithoutUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
}

func checkFileSignature(path, signature string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data := make([]byte, len(signature))
	if _, err := io.ReadFull(f, data); err != nil || string(data) != signature {
		return errors.New("文件内容与扩展名不符，或文件已损坏")
	}
	return nil
}

func (a *application) openDocumentPreview(d *document) {
	if d.loading || d.previewError != "" {
		return
	}
	switch {
	case d.systemPreview:
		a.openNativePreview(d)
	case d.kind == kindMedia || d.kind == kindSystem:
		a.openNativePreview(d)
	case d.kind == kindPDF:
		if err := a.openPDF(d.path); err != nil {
			d.previewError = err.Error()
		}
	case d.kind == kindEPUB:
		a.openEPUB(d)
	case d.kind == kindDocument && !d.showSource, d.kind == kindHTML || d.kind == kindOffice || d.kind == kindSVG || d.imageAnimated || (d.kind == kindMarkdown && d.htmlRich):
		a.openHTMLPreview(d)
	}
}

func (a *application) previewMessage(c *ui.Context, d *document) bool {
	if d.loading {
		ui.Column(c).Grow(1).Center().Gap(12).Children(func() {
			ui.Text(c, a.tr("Opening preview…")).TextColor(c.Theme().TextMuted)
			ui.Text(c, filepath.Base(d.path)).SingleLine()
		})
		return true
	}
	if d.previewError == "" {
		return false
	}
	ui.Column(c).Grow(1).Center().Gap(12).Padding(28).Children(func() {
		ui.Text(c, a.tr("Preview unavailable")).FontSize(19).Bold()
		ui.Text(c, d.previewError).MaxWidth(520).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, a.tr("Try again")).Clicked() {
				a.closeNativePreview()
				a.closeHTMLPreview()
				d.imageIndex = 0
				if d.systemPreview {
					d.previewError = ""
					a.openNativePreview(d)
				} else {
					a.loadDocument(d)
				}
			}
			if ui.Button(c, a.tr("Open in default app")).Clicked() {
				go func() {
					if err := mygo.Shell.OpenPath(d.path); err != nil {
						a.win.Update(func() { a.status = "无法打开：" + err.Error() })
					}
				}()
			}
			if ui.Button(c, a.tr("Show in Finder")).Clicked() {
				mygo.Shell.ShowItemInFolder(d.path)
			}
		})
	})
	return true
}

func (a *application) imageView(c *ui.Context, d *document) {
	if d.picture == nil {
		return
	}
	if d.imageZoom == 0 {
		ui.Image(c, d.picture).Fill().Fit(ui.Contain).Padding(22)
		return
	}
	w, h := ui.ImageSize(d.picture)
	ui.ScrollBoth(c).Fill().TrackScroll(&d.imageScroll).Children(func() {
		ui.Image(c, d.picture).Size(w*d.imageZoom, h*d.imageZoom).MinWidth(w * d.imageZoom).MinHeight(h * d.imageZoom).Fit(ui.Contain)
	})
}

func (a *application) zoomImage(scale float32) {
	d := a.doc
	if d == nil || (d.kind != kindImage && d.kind != kindSVG) {
		return
	}
	d.imageZoom = scale
	if (d.kind == kindSVG || d.imageAnimated) && a.htmlView != nil {
		page := a.htmlView.Page()
		go page.Eval(fmt.Sprintf("window.catviewImageZoom && window.catviewImageZoom(%g)", scale))
	}
	a.win.Invalidate()
}

func (a *application) adjustContentSize(delta float32) {
	d := a.doc
	if d == nil || (d.kind != kindImage && d.kind != kindSVG) {
		a.adjustFont(delta)
		return
	}
	scale := d.imageZoom
	if scale == 0 {
		scale = 1
	}
	if delta > 0 {
		scale *= 1.25
	} else {
		scale /= 1.25
	}
	a.zoomImage(max(float32(0.1), min(float32(8), scale)))
}
