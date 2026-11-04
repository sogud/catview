package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	stdhtml "html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
var cssLength = regexp.MustCompile(`^(\d{1,4}(?:\.\d{1,2})?)(px|pt|em|rem|%)$`)

func renderMarkdownMarkup(source string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldhtml.WithUnsafe()),
	)
	var out bytes.Buffer
	context := parser.NewContext(parser.WithIDs(&headingIDs{used: make(map[string]bool)}))
	if err := md.Convert([]byte(source), &out, parser.WithContext(context)); err != nil {
		return "", err
	}
	return out.String(), nil
}

var previewTags = map[string]bool{
	"p": true, "div": true, "span": true, "section": true, "article": true,
	"b": true, "strong": true, "i": true, "em": true, "u": true, "s": true,
	"del": true, "ins": true, "mark": true, "small": true, "sup": true, "sub": true,
	"code": true, "pre": true, "kbd": true, "br": true, "hr": true,
	"blockquote": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true, "td": true, "th": true,
	"img": true, "a": true, "figure": true, "figcaption": true,
	"details": true, "summary": true, "center": true, "ruby": true, "rt": true, "rp": true,
}
var blockedTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true, "embed": true,
	"form": true, "input": true, "button": true, "textarea": true, "select": true,
	"meta": true, "link": true, "base": true, "head": true, "svg": true,
	"video": true, "audio": true, "canvas": true, "template": true,
}
var voidTags = map[string]bool{"img": true, "br": true, "hr": true}

func safeCSSDeclarations(value string) string {
	var result []string
	for _, item := range strings.Split(value, ";") {
		name, raw, ok := strings.Cut(item, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		raw = strings.TrimSpace(raw)
		switch name {
		case "text-align":
			switch strings.ToLower(raw) {
			case "left", "right", "center", "justify", "start", "end":
				result = append(result, name+":"+strings.ToLower(raw))
			}
		case "color", "background-color":
			if hexColor.MatchString(raw) {
				result = append(result, name+":"+raw)
			}
		case "font-weight":
			switch strings.ToLower(raw) {
			case "bold", "normal", "bolder", "lighter", "100", "200", "300", "400", "500", "600", "700", "800", "900":
				result = append(result, name+":"+strings.ToLower(raw))
			}
		case "font-style":
			switch strings.ToLower(raw) {
			case "italic", "normal", "oblique":
				result = append(result, name+":"+strings.ToLower(raw))
			}
		case "text-decoration":
			switch strings.ToLower(raw) {
			case "underline", "line-through", "none":
				result = append(result, name+":"+strings.ToLower(raw))
			}
		case "width", "max-width", "height", "font-size":
			if cssLength.MatchString(raw) {
				result = append(result, name+":"+raw)
			}
		}
	}
	return strings.Join(result, ";")
}

func (a *application) localPreviewImage(sourceFile, imageRef, scopeRoot string) string {
	if imageRef == "" || strings.ContainsAny(imageRef, "\x00\\") || strings.HasPrefix(imageRef, "//") {
		return ""
	}
	parsed, err := url.Parse(imageRef)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || filepath.IsAbs(parsed.Path) {
		return ""
	}
	ref, err := url.PathUnescape(parsed.Path)
	if err != nil || ref == "" || filepath.IsAbs(ref) {
		return ""
	}
	path := filepath.Join(filepath.Dir(sourceFile), filepath.FromSlash(ref))
	if !pathInScope(scopeRoot, path) {
		return ""
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxImageBytes {
		return ""
	}
	typ := ""
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		typ = "image/png"
	case ".jpg", ".jpeg":
		typ = "image/jpeg"
	case ".gif":
		typ = "image/gif"
	case ".webp":
		typ = "image/webp"
	default:
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// sanitizePreviewHTML allows typography, tables and local images while
// removing scripts, active content and attributes with side effects.
func (a *application) sanitizePreviewHTML(fragment, sourceFile, scopeRoot string) (string, error) {
	ctx := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := nethtml.ParseFragment(strings.NewReader(fragment), ctx)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	var emit func(*nethtml.Node)
	emit = func(n *nethtml.Node) {
		switch n.Type {
		case nethtml.TextNode:
			out.WriteString(stdhtml.EscapeString(n.Data))
			return
		case nethtml.ElementNode:
			tag := strings.ToLower(n.Data)
			if blockedTags[tag] {
				return
			}
			if !previewTags[tag] {
				for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
					emit(ch)
				}
				return
			}
			if tag == "code" && n.Parent != nil && n.Parent.Data == "pre" && emitHighlightedCode(&out, n) {
				return
			}
			if tag == "img" {
				src := ""
				for _, attr := range n.Attr {
					if strings.EqualFold(attr.Key, "src") {
						src = a.localPreviewImage(sourceFile, attr.Val, scopeRoot)
					}
				}
				if src == "" {
					return
				}
				out.WriteString(`<img src="` + src + `"`)
				for _, attr := range n.Attr {
					if attr.Key == "alt" || attr.Key == "title" {
						out.WriteString(" " + attr.Key + `="` + stdhtml.EscapeString(attr.Val) + `"`)
					}
				}
				out.WriteString(">")
				return
			}
			out.WriteByte('<')
			out.WriteString(tag)
			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				value := ""
				switch key {
				case "style":
					value = safeCSSDeclarations(attr.Val)
				case "align":
					if attr.Val == "center" || attr.Val == "right" || attr.Val == "left" {
						key = "style"
						value = "text-align:" + attr.Val
					}
				case "href":
					if tag != "a" {
						continue
					}
					value = previewLink(sourceFile, attr.Val)
				case "id", "name":
					if (key == "id" || tag == "a") && len(attr.Val) <= 256 && !strings.ContainsAny(attr.Val, "\x00\r\n\t ") {
						value = attr.Val
					}
				case "title", "alt", "lang", "dir":
					value = attr.Val
					if key == "dir" && value != "ltr" && value != "rtl" && value != "auto" {
						value = ""
					}
				case "colspan", "rowspan":
					if n, e := strconv.Atoi(attr.Val); e == nil && n > 0 && n <= 20 {
						value = strconv.Itoa(n)
					}
				case "open":
					if tag == "details" {
						out.WriteString(" open")
					}
					continue
				case "class":
					if strings.HasPrefix(attr.Val, "language-") && len(attr.Val) < 50 {
						value = attr.Val
					}
				default:
					continue
				}
				if value == "" {
					continue
				}
				out.WriteByte(' ')
				out.WriteString(key)
				out.WriteString(`="`)
				out.WriteString(stdhtml.EscapeString(value))
				out.WriteByte('"')
			}
			if tag == "a" {
				out.WriteString(` rel="noopener noreferrer"`)
			}
			out.WriteByte('>')
			if voidTags[tag] {
				return
			}
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				emit(ch)
			}
			out.WriteString("</")
			out.WriteString(tag)
			out.WriteByte('>')
		}
	}
	for _, n := range nodes {
		emit(n)
	}
	return out.String(), nil
}

const previewCSS = `
:root{color-scheme:light dark;--paper:var(--paper-light,#FBFAF7);--ink:var(--ink-light,#2B3034);--secondary:var(--link-light,#526C7B);--line:var(--line-light,#E0E4E1);--panel:var(--panel-light,#F4F4F2)}
@media(prefers-color-scheme:dark){:root{--paper:var(--paper-dark,#202225);--ink:var(--ink-dark,#EDF0F1);--secondary:var(--link-dark,#B6CAD5);--line:var(--line-dark,#3C4144);--panel:var(--panel-dark,#292B2E)}}
:root[data-theme=light]{color-scheme:light;--paper:var(--paper-light,#FBFAF7);--ink:var(--ink-light,#2B3034);--secondary:var(--link-light,#526C7B);--line:var(--line-light,#E0E4E1);--panel:var(--panel-light,#F4F4F2)}
:root[data-theme=dark]{color-scheme:dark;--paper:var(--paper-dark,#202225);--ink:var(--ink-dark,#EDF0F1);--secondary:var(--link-dark,#B6CAD5);--line:var(--line-dark,#3C4144);--panel:var(--panel-dark,#292B2E)}
*{box-sizing:border-box}
html,body{padding:0;margin:0;width:100%;min-height:100%}
body{background:var(--paper);color:var(--ink);font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Helvetica Neue",sans-serif;-webkit-font-smoothing:antialiased}
main{margin:auto;max-width:var(--reader-width);padding:32px 42px 85px;overflow-wrap:break-word}
h1,h2,h3,h4,h5,h6{font-weight:650;line-height:1.35;margin:1.35em 0 .55em}
main>:first-child{margin-top:0}
h1{font-size:1.85em}h2{font-size:1.45em}h3{font-size:1.2em}h4{font-size:1.08em}
p{margin:.7em 0 1em}a{color:var(--secondary);text-decoration:underline;text-underline-offset:.16em}
blockquote{border-left:3px solid var(--line);margin:1.2em 0;padding:.1em 1.1em;color:var(--secondary)}
pre{font-size:1em;line-height:inherit;padding:14px 17px;border-radius:7px;background:var(--paper);border:1px solid var(--line);white-space:pre;overflow:auto;tab-size:4}
code,kbd{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
:not(pre)>code,kbd{background:rgba(100,124,145,.10);border-radius:3px;padding:.1em .3em}
table{max-width:100%;width:100%;border-collapse:collapse;margin:1.2em 0;font-size:.92em}
td,th{padding:9px 13px;border:1px solid var(--line);text-align:left;vertical-align:top}
th{background:rgba(100,124,145,.06);font-weight:600}
ul,ol{padding-left:1.8em}li{margin:.2em 0}
img{max-width:100%;max-height:480px;width:auto;height:auto;object-fit:contain;display:block;margin:1.2em auto}
figure{margin:1.5em 0;text-align:center}figcaption{font-size:.85em;color:var(--secondary)}
hr{height:0;border:0;border-top:1px solid var(--line);margin:1.8em 0}
details{border:1px solid var(--line);border-radius:6px;padding:9px 13px}
summary{cursor:pointer}
[id]{scroll-margin-top:24px}
.cv-diagram{margin:24px 0;padding:20px 16px;overflow:auto;border:1px solid var(--line);border-radius:8px;background:var(--paper)}
.cv-diagram svg{display:block;margin:auto;height:auto;max-width:none}
.cv-diagram-error{margin:12px 0 0;font-size:.85em;color:var(--secondary)}
mark{background:#f4e4ac;padding:0 .12em}
.cv-slide{background:#fff;color:#222;border:1px solid #ddd;position:relative;aspect-ratio:var(--slide-aspect,16/9);width:100%;overflow:hidden;margin:0 0 30px;box-shadow:0 3px 14px #0001}
.cv-slide *{box-sizing:border-box}.cv-slide img{max-height:none;margin:0}.cv-slide p{margin:0}
.cv-shape{position:absolute;overflow:hidden;white-space:pre-wrap;overflow-wrap:anywhere}
.cv-slide-title{font-size:.78em;font-weight:600;color:var(--secondary);margin:8px 0}
@media(max-width:680px){main{padding:22px 19px 55px}}
`

func (a *application) wrapHTMLPreview(content string, presentation bool, script, nonce string) string {
	mode := "auto"
	if a.themeMode == 1 {
		mode = "light"
	} else if a.themeMode == 2 {
		mode = "dark"
	}
	width := a.preferredTextWidth()
	if presentation {
		width = 1070
	}
	style := fmt.Sprintf("font-size:%.0fpx;line-height:%.2f;--reader-width:%.0fpx", a.fontSize, a.lineHeight, width)
	palette := a.palette()
	scriptPolicy := "'none'"
	if nonce != "" {
		scriptPolicy = "'nonce-" + nonce + "'"
	}
	return `<!doctype html><html data-theme="` + mode + `" style="` + a.themeCSSVariables() + `--link-light:` + palette.lightLink + `;--link-dark:` + palette.darkLink + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src ` + scriptPolicy + `; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"><style>` + previewCSS + codeTokenCSS + `</style></head><body style="` + style + `"><main>` + content + `</main>` + script + `</body></html>`
}

func (a *application) generateHTMLPreview(d *document) (string, error) {
	if d == nil {
		return "", fmt.Errorf("no document")
	}
	switch d.kind {
	case kindMarkdown:
		html, err := renderMarkdownMarkup(d.source)
		if err != nil {
			return "", err
		}
		body, err := a.sanitizePreviewHTML(html, d.path, d.scopeRoot)
		if err != nil {
			return "", err
		}
		script, nonce, err := a.markdownScripts(body, d.fragment)
		if err != nil {
			return "", err
		}
		return a.wrapHTMLPreview(body, false, script, nonce), nil
	case kindHTML:
		body, err := a.sanitizePreviewHTML(d.source, d.path, d.scopeRoot)
		if err != nil {
			return "", err
		}
		return a.wrapHTMLPreview(body, false, "", ""), nil
	case kindOffice:
		return a.wrapHTMLPreview(d.officeHTML, d.officePresentation, "", ""), nil
	}
	return "", fmt.Errorf("unsupported preview type")
}

func (a *application) openHTMLPreview(d *document) {
	if d == nil {
		return
	}
	html, err := a.generateHTMLPreview(d)
	if err != nil {
		a.status = "预览失败：" + err.Error()
		return
	}
	if a.htmlView == nil {
		view, err := a.win.NewWebView(mygo.WebViewOptions{
			Page:            mygo.PageOptions{Untrusted: true},
			BackgroundColor: a.webBackground(),
		})
		if err != nil {
			a.status = "HTML 视图创建失败：" + err.Error()
			return
		}
		a.htmlView = view
		view.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			parsed, err := url.Parse(e.URL)
			if err != nil {
				e.PreventDefault()
				return
			}
			if parsed.Scheme == "https" || parsed.Scheme == "http" || parsed.Scheme == "mailto" {
				e.PreventDefault()
				if e.UserInitiated {
					go mygo.Shell.OpenExternal(e.URL)
				}
				return
			}
			if parsed.Scheme == "catview" && parsed.Host == "open" {
				e.PreventDefault()
				if e.UserInitiated && a.doc != nil {
					a.openPreviewLink(parsed.Query().Get("path"), parsed.Query().Get("fragment"), a.doc.scopeRoot)
				}
				return
			}
			if strings.HasPrefix(e.URL, "about:blank") {
				return
			}
			e.PreventDefault()
		})
	}
	a.htmlView.Page().LoadHTML(html, "")
	a.win.Invalidate()
}

func (a *application) closeHTMLPreview() {
	if a.htmlView != nil {
		a.htmlView.Destroy()
		a.htmlView = nil
	}
}

func (a *application) updateHTMLPreview() {
	d := a.doc
	if d == nil || !((d.kind == kindMarkdown && d.htmlRich) || d.kind == kindHTML || d.kind == kindOffice) {
		return
	}
	a.openHTMLPreview(d)
}
