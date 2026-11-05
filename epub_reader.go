package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

//go:embed epub-viewer.html
var epubViewerTemplate string

type epubPosition struct {
	Ready    bool    `json:"ready,omitempty"`
	Chapter  int     `json:"chapter"`
	Fraction float64 `json:"fraction"`
}

type epubPayload struct {
	*epubBook
	FontSize    float32      `json:"fontSize"`
	ThemeMode   int          `json:"themeMode"`
	LineHeight  float32      `json:"lineHeight"`
	ReaderWidth float32      `json:"readerWidth"`
	Colors      string       `json:"colors"`
	LinkLight   string       `json:"linkLight"`
	LinkDark    string       `json:"linkDark"`
	Position    epubPosition `json:"position"`
}

func (a *application) epubPositionPath(bookFile string) string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(bookFile))
	return filepath.Join(dir, "epub-"+hex.EncodeToString(sum[:12])+".json")
}

func (a *application) readEPUBPosition(bookFile string) epubPosition {
	filename := a.epubPositionPath(bookFile)
	if filename == "" {
		return epubPosition{}
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return epubPosition{}
	}
	var pos epubPosition
	if json.Unmarshal(data, &pos) != nil {
		return epubPosition{}
	}
	return pos
}

func (a *application) writeEPUBPosition(bookFile string, pos epubPosition) {
	if !pos.Ready || pos.Chapter < 0 || math.IsNaN(pos.Fraction) || math.IsInf(pos.Fraction, 0) || pos.Fraction < 0 || pos.Fraction > 1 {
		return
	}
	filename := a.epubPositionPath(bookFile)
	if filename == "" {
		return
	}
	data, err := json.Marshal(pos)
	if err != nil {
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(filename), ".epub-progress-*")
	if err != nil {
		return
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return
	}
	if err := temp.Close(); err != nil {
		return
	}
	_ = os.Rename(temp.Name(), filename)
}

func renderEPUBHTML(book *epubBook, fontSize float32, themeMode int, lineHeight, readerWidth float32, linkLight, linkDark, colors string, position epubPosition) (string, error) {
	payload, err := json.Marshal(epubPayload{
		epubBook: book, FontSize: fontSize, ThemeMode: themeMode,
		LineHeight: lineHeight, ReaderWidth: readerWidth,
		LinkLight: linkLight, LinkDark: linkDark, Colors: colors, Position: position,
	})
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 18)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	token := base64.RawStdEncoding.EncodeToString(nonce)
	page := strings.ReplaceAll(epubViewerTemplate, "__NONCE__", token)
	page = strings.Replace(page, "__BOOK_JSON__", string(payload), 1)
	return page, nil
}

func (a *application) openEPUB(d *document) {
	if d == nil || d.book == nil {
		return
	}
	a.closeEPUB()
	page, err := a.win.NewWebView(mygo.WebViewOptions{
		Page:            mygo.PageOptions{Untrusted: true},
		BackgroundColor: a.webBackground(),
	})
	if err != nil {
		a.status = "EPUB 视图无法创建：" + err.Error()
		return
	}
	palette := a.palette()
	html, err := renderEPUBHTML(d.book, a.fontSize, a.themeMode, a.lineHeight, a.preferredTextWidth(),
		palette.lightLink, palette.darkLink, a.themeCSSVariables(), a.readEPUBPosition(d.path))
	if err != nil {
		page.Destroy()
		a.status = "EPUB 页面无法生成：" + err.Error()
		return
	}
	a.epubView = page
	html = strings.Replace(html, "</head>", "<style>"+a.fontCSS()+"</style></head>", 1)
	page.Page().LoadHTML(html, "")
	a.win.Invalidate()

	ctx, cancel := context.WithCancel(context.Background())
	a.epubCancel = cancel
	go a.pollEPUBPosition(ctx, d.path, page)
}

func (a *application) closeEPUB() {
	if a.epubCancel != nil {
		a.epubCancel()
		a.epubCancel = nil
	}
	if a.epubView != nil {
		a.epubView.Destroy()
		a.epubView = nil
	}
}

func (a *application) pollEPUBPosition(ctx context.Context, bookFile string, view *mygo.WebView) {
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	var last epubPosition
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// MyGo evaluates on the webview's own main page; untrusted
			// book markup cannot call Go or access local files.
			value, err := mygo.EvalAs[epubPosition](view.Page(), "window.catviewReader ? Object.assign({ready:true}, window.catviewReader.position()) : null")
			if err != nil || !value.Ready || value == last {
				continue
			}
			last = value
			a.writeEPUBPosition(bookFile, value)
		}
	}
}

func (a *application) epubCommand(method string, args ...string) {
	if a.doc == nil || a.doc.kind != kindEPUB || a.epubView == nil {
		return
	}
	switch method {
	case "nextPage", "previousPage", "nextChapter", "previousChapter", "goTo", "setFont", "setTheme", "setLayout", "setAccent", "setFamilies":
	default:
		return
	}
	target := a.epubView
	expr := "window.catviewReader?." + method + "(" + strings.Join(args, ",") + ")"
	go func() {
		if _, err := target.Page().Eval(expr); err != nil {
			// The view might have been closed while the command was queued.
			return
		}
	}()
}

func (a *application) epubGoTo(chapter int, anchor string) {
	encoded, err := json.Marshal(anchor)
	if err != nil {
		return
	}
	a.epubCommand("goTo", strconv.Itoa(chapter), string(encoded))
}

func (a *application) epubNextPage()        { a.epubCommand("nextPage") }
func (a *application) epubPreviousPage()    { a.epubCommand("previousPage") }
func (a *application) epubNextChapter()     { a.epubCommand("nextChapter") }
func (a *application) epubPreviousChapter() { a.epubCommand("previousChapter") }

func (a *application) epubSetFont() {
	a.epubCommand("setFont", strconv.FormatFloat(float64(a.fontSize), 'f', -1, 32))
}

func (a *application) epubSetTheme() {
	a.epubCommand("setTheme", strconv.Itoa(a.themeMode))
}

func (a *application) epubSetAccent() {
	palette := a.palette()
	light, errLight := json.Marshal(palette.lightLink)
	dark, errDark := json.Marshal(palette.darkLink)
	if errLight != nil || errDark != nil {
		return
	}
	colors, _ := json.Marshal(a.themeCSSVariables())
	a.epubCommand("setAccent", string(light), string(dark), string(colors))
}

func (a *application) epubSetLayout() {
	a.epubCommand("setLayout",
		strconv.FormatFloat(float64(a.lineHeight), 'f', 2, 32),
		strconv.FormatFloat(float64(a.preferredTextWidth()), 'f', 0, 32),
	)
}

func (a *application) epubReadMenuItems() []*mygo.MenuItem {
	if a.doc == nil || a.doc.kind != kindEPUB || a.doc.book == nil {
		return nil
	}
	action := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { fn() }
	}
	items := []*mygo.MenuItem{
		{Label: "上一屏", Accelerator: "Alt+Left", Click: action(a.epubPreviousPage)},
		{Label: "下一屏", Accelerator: "Alt+Right", Click: action(a.epubNextPage)},
		{Label: "上一章", Click: action(a.epubPreviousChapter)},
		{Label: "下一章", Click: action(a.epubNextChapter)},
		mygo.Separator(),
	}
	var toc []*mygo.MenuItem
	for _, item := range a.doc.book.Contents {
		item := item
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		runes := []rune(title)
		if len(runes) > 55 {
			title = string(runes[:55]) + "…"
		}
		title = strings.Repeat("  ", max(0, min(4, item.Depth))) + title
		toc = append(toc, &mygo.MenuItem{
			Label: title,
			Click: action(func() { a.epubGoTo(item.Chapter, item.Anchor) }),
		})
	}
	if len(toc) == 0 {
		toc = []*mygo.MenuItem{{Label: "没有章节目录", Disabled: true}}
	}
	items = append(items, &mygo.MenuItem{Label: "目录", Submenu: toc})
	return items
}

func (a *application) epubSetFamilies() {
	reading, _ := json.Marshal(cssFontFamily(a.readingFont, "ui-serif,serif"))
	code, _ := json.Marshal(cssFontFamily(a.codeFont, "ui-monospace,monospace"))
	a.epubCommand("setFamilies", string(reading), string(code))
}
