package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	epubMaxArchiveBytes = 160 << 20
	epubMaxTextBytes    = 28 << 20
	epubMaxChapterBytes = 4 << 20
	epubMaxImageBytes   = 8 << 20
	epubMaxChapters     = 1000
	epubMaxFiles        = 15000
)

type epubBook struct {
	Title     string        `json:"title"`
	Language  string        `json:"language"`
	Direction string        `json:"direction"`
	Vertical  bool          `json:"vertical"`
	Chapters  []epubChapter `json:"chapters"`
	Contents  []epubTOCItem `json:"contents"`
}

type epubChapter struct {
	Title string `json:"title"`
	HTML  string `json:"html"`
	Path  string `json:"-"`
}

type epubTOCItem struct {
	Title   string `json:"title"`
	Chapter int    `json:"chapter"`
	Anchor  string `json:"anchor,omitempty"`
	Depth   int    `json:"depth,omitempty"`
}

type epubContainer struct {
	RootFiles []struct {
		Path string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Metadata struct {
		Titles    []string `xml:"title"`
		Languages []string `xml:"language"`
		Meta      []struct {
			Property string `xml:"property,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []epubItem `xml:"manifest>item"`
	Spine    struct {
		TOC       string `xml:"toc,attr"`
		Direction string `xml:"page-progression-direction,attr"`
		Items     []struct {
			ID     string `xml:"idref,attr"`
			Linear string `xml:"linear,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type epubItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type epubArchive struct {
	files  map[string]*zip.File
	images map[string]string
}

func decodeEPUBXML(data []byte, target any) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	return dec.Decode(target)
}

func safeEPUBPath(base, href string) (string, string, error) {
	if strings.ContainsAny(href, "\x00\\") {
		return "", "", errors.New("invalid EPUB path")
	}
	u, err := url.Parse(href)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || strings.HasPrefix(href, "//") || strings.HasPrefix(u.Path, "/") {
		return "", "", errors.New("external EPUB resource")
	}
	name, err := url.PathUnescape(u.Path)
	if err != nil || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") {
		return "", "", errors.New("invalid EPUB resource")
	}
	if name == "" {
		name = path.Base(base)
	}
	normalized := path.Clean(path.Join(path.Dir(base), name))
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", "", errors.New("EPUB path traversal")
	}
	fragment, err := url.PathUnescape(u.Fragment)
	if err != nil {
		return "", "", err
	}
	return normalized, fragment, nil
}

func (a *epubArchive) read(name string, maxSize int64) ([]byte, error) {
	file := a.files[name]
	if file == nil {
		return nil, fmt.Errorf("EPUB resource missing: %s", name)
	}
	if file.UncompressedSize64 > uint64(maxSize) {
		return nil, fmt.Errorf("EPUB resource too large: %s", name)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, errors.New("EPUB resource exceeds size limit")
	}
	return data, nil
}

func (a *epubArchive) imageData(bookPath string, ref string) (string, bool) {
	name, _, err := safeEPUBPath(bookPath, ref)
	if err != nil {
		return "", false
	}
	if cached, ok := a.images[name]; ok {
		return cached, cached != ""
	}
	typ := ""
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		typ = "image/png"
	case ".jpg", ".jpeg":
		typ = "image/jpeg"
	case ".gif":
		typ = "image/gif"
	case ".webp":
		typ = "image/webp"
	}
	if typ == "" {
		a.images[name] = ""
		return "", false
	}
	data, err := a.read(name, epubMaxImageBytes)
	if err != nil {
		a.images[name] = ""
		return "", false
	}
	value := "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data)
	a.images[name] = value
	return value, true
}

func parseEPUB(filename string) (*epubBook, error) {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("EPUB is not a readable ZIP archive: %w", err)
	}
	defer zipReader.Close()

	if len(zipReader.File) > epubMaxFiles {
		return nil, errors.New("EPUB contains too many files")
	}
	a := &epubArchive{files: make(map[string]*zip.File), images: make(map[string]string)}
	var total uint64
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if path.Clean(name) != name || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
			return nil, errors.New("EPUB contains unsafe archive paths")
		}
		total += f.UncompressedSize64
		if total > epubMaxArchiveBytes {
			return nil, errors.New("EPUB extracted size exceeds 160 MB")
		}
		a.files[name] = f
	}
	if _, ok := a.files["META-INF/rights.xml"]; ok {
		return nil, errors.New("DRM-protected EPUB is not supported")
	}
	// Encrypted resources cannot be rendered safely without interpreting
	// the encryption metadata, so reject them rather than show corrupt text.
	if _, ok := a.files["META-INF/encryption.xml"]; ok {
		return nil, errors.New("encrypted EPUB resources are not supported")
	}
	containerData, err := a.read("META-INF/container.xml", 256<<10)
	if err != nil {
		return nil, err
	}
	var container epubContainer
	if err := decodeEPUBXML(containerData, &container); err != nil || len(container.RootFiles) == 0 {
		return nil, errors.New("EPUB container.xml is invalid")
	}
	opfPath, _, err := safeEPUBPath("root", container.RootFiles[0].Path)
	if err != nil {
		return nil, err
	}
	opfData, err := a.read(opfPath, 4<<20)
	if err != nil {
		return nil, err
	}
	var pkg epubPackage
	if err := decodeEPUBXML(opfData, &pkg); err != nil {
		return nil, fmt.Errorf("invalid EPUB package: %w", err)
	}
	manifest := make(map[string]epubItem)
	var navigation, ncx string
	for _, item := range pkg.Manifest {
		if item.ID == "" {
			continue
		}
		manifest[item.ID] = item
		if strings.Contains(" "+item.Properties+" ", " nav ") {
			navigation, _, _ = safeEPUBPath(opfPath, item.Href)
		}
		if item.ID == pkg.Spine.TOC || item.MediaType == "application/x-dtbncx+xml" {
			ncx, _, _ = safeEPUBPath(opfPath, item.Href)
		}
	}
	book := &epubBook{Language: "en", Direction: "ltr"}
	if len(pkg.Metadata.Titles) > 0 {
		book.Title = strings.TrimSpace(pkg.Metadata.Titles[0])
	}
	if len(pkg.Metadata.Languages) > 0 {
		book.Language = strings.TrimSpace(pkg.Metadata.Languages[0])
	}
	if pkg.Spine.Direction == "rtl" {
		book.Direction = "rtl"
	}
	if strings.HasPrefix(strings.ToLower(book.Language), "ar") || strings.HasPrefix(strings.ToLower(book.Language), "he") || strings.HasPrefix(strings.ToLower(book.Language), "fa") || strings.HasPrefix(strings.ToLower(book.Language), "ur") {
		book.Direction = "rtl"
	}
	for _, meta := range pkg.Metadata.Meta {
		if meta.Property == "rendition:layout" && strings.TrimSpace(meta.Value) == "pre-paginated" {
			return nil, errors.New("fixed-layout EPUB is not supported yet")
		}
	}
	for _, item := range pkg.Manifest {
		if item.MediaType != "text/css" {
			continue
		}
		name, _, err := safeEPUBPath(opfPath, item.Href)
		if err != nil {
			continue
		}
		css, err := a.read(name, 256<<10)
		if err == nil && regexp.MustCompile(`(?i)(?:-epub-)?writing-mode\s*:\s*vertical-(?:rl|lr)`).Match(css) {
			book.Vertical = true
			break
		}
	}
	chapterPaths := make(map[string]int)
	var rawChapters [][]byte
	for _, ref := range pkg.Spine.Items {
		if len(book.Chapters) >= epubMaxChapters {
			return nil, errors.New("EPUB contains too many spine chapters")
		}
		item, ok := manifest[ref.ID]
		if !ok || (item.MediaType != "application/xhtml+xml" && item.MediaType != "text/html") || ref.Linear == "no" {
			continue
		}
		name, _, err := safeEPUBPath(opfPath, item.Href)
		if err != nil {
			return nil, err
		}
		raw, err := a.read(name, epubMaxChapterBytes)
		if err != nil {
			return nil, err
		}
		chapterPaths[name] = len(book.Chapters)
		book.Chapters = append(book.Chapters, epubChapter{Path: name, Title: path.Base(name)})
		rawChapters = append(rawChapters, raw)
	}
	if len(book.Chapters) == 0 {
		return nil, errors.New("EPUB has no readable XHTML spine chapters")
	}
	var textSize int
	for i, raw := range rawChapters {
		body, title, err := sanitizeEPUBChapter(raw, book.Chapters[i].Path, i, a, chapterPaths)
		if err != nil {
			return nil, err
		}
		textSize += len(body)
		if textSize > epubMaxTextBytes {
			return nil, errors.New("EPUB content exceeds reader size limit")
		}
		book.Chapters[i].HTML = body
		if title != "" {
			book.Chapters[i].Title = title
		}
	}
	if navigation != "" {
		if raw, err := a.read(navigation, 3<<20); err == nil {
			book.Contents = readEPUBNavigation(raw, navigation, chapterPaths)
		}
	}
	if len(book.Contents) == 0 && ncx != "" {
		if raw, err := a.read(ncx, 3<<20); err == nil {
			book.Contents = readEPUBNCX(raw, ncx, chapterPaths)
		}
	}
	if len(book.Contents) == 0 {
		for i, chapter := range book.Chapters {
			book.Contents = append(book.Contents, epubTOCItem{Title: chapter.Title, Chapter: i})
		}
	}
	return book, nil
}

var epubAllowedTags = map[string]bool{
	"p": true, "div": true, "span": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"em": true, "strong": true, "i": true, "b": true, "u": true, "s": true, "small": true, "blockquote": true,
	"pre": true, "code": true, "ol": true, "ul": true, "li": true, "table": true, "thead": true, "tbody": true,
	"tfoot": true, "tr": true, "td": true, "th": true, "br": true, "hr": true, "figure": true, "figcaption": true,
	"ruby": true, "rt": true, "rp": true, "bdi": true, "bdo": true, "sup": true, "sub": true, "abbr": true,
	"del": true, "ins": true, "section": true, "article": true, "main": true, "a": true, "img": true,
	"dl": true, "dt": true, "dd": true, "cite": true, "q": true, "time": true,
}

var epubDiscardTags = map[string]bool{
	"script": true, "style": true, "svg": true, "iframe": true, "object": true, "embed": true,
	"form": true, "input": true, "button": true, "textarea": true, "select": true, "video": true,
	"audio": true, "source": true, "track": true, "canvas": true, "applet": true, "math": true,
	"meta": true, "link": true, "template": true, "noscript": true,
}

func epubNodeAttr(node *nethtml.Node, names ...string) string {
	for _, attr := range node.Attr {
		for _, name := range names {
			if attr.Key == name {
				return attr.Val
			}
		}
	}
	return ""
}

func epubNodeText(node *nethtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == nethtml.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(epubNodeText(child))
	}
	return builder.String()
}

func findEPUBElement(node *nethtml.Node, tag string) *nethtml.Node {
	if node.Type == nethtml.ElementNode && node.Data == tag {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findEPUBElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func sanitizeEPUBChapter(raw []byte, chapterPath string, index int, a *epubArchive, paths map[string]int) (string, string, error) {
	reader, err := charset.NewReader(bytes.NewReader(raw), "")
	if err != nil {
		reader = bytes.NewReader(raw)
	}
	tree, err := nethtml.Parse(reader)
	if err != nil {
		return "", "", fmt.Errorf("invalid EPUB chapter %s: %w", chapterPath, err)
	}
	body := findEPUBElement(tree, "body")
	if body == nil {
		body = tree
	}
	var result strings.Builder
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			result.WriteString(stdhtml.EscapeString(node.Data))
			return
		}
		if node.Type != nethtml.ElementNode {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
			return
		}
		name := strings.ToLower(node.Data)
		if epubDiscardTags[name] {
			return
		}
		allowed := epubAllowedTags[name]
		if allowed {
			if name == "img" {
				src, ok := a.imageData(chapterPath, epubNodeAttr(node, "src"))
				if !ok {
					return
				}
				result.WriteString("<img src=\"")
				result.WriteString(stdhtml.EscapeString(src))
				result.WriteString("\"")
			} else {
				result.WriteByte('<')
				result.WriteString(name)
			}
			if id := epubNodeAttr(node, "id", "name"); id != "" && len(id) <= 180 {
				result.WriteString(" id=\"cv-" + strconv.Itoa(index) + "-" + stdhtml.EscapeString(id) + "\"")
			}
			if lang := epubNodeAttr(node, "lang"); lang != "" && len(lang) <= 40 {
				result.WriteString(" lang=\"" + stdhtml.EscapeString(lang) + "\"")
			}
			if direction := epubNodeAttr(node, "dir"); direction == "rtl" || direction == "ltr" || direction == "auto" {
				result.WriteString(" dir=\"" + direction + "\"")
			}
			if name == "a" {
				ref := epubNodeAttr(node, "href")
				if target, fragment, err := safeEPUBPath(chapterPath, ref); err == nil {
					if idx, ok := paths[target]; ok {
						anchor := fmt.Sprintf("#cv-%d", idx)
						if fragment != "" {
							anchor += "-" + fragment
						}
						result.WriteString(" href=\"" + stdhtml.EscapeString(anchor) + "\"")
					}
				}
			}
			if name == "img" {
				if alt := epubNodeAttr(node, "alt"); alt != "" {
					result.WriteString(" alt=\"" + stdhtml.EscapeString(alt) + "\"")
				}
				result.WriteString(" loading=\"lazy\"")
			}
			if name == "th" || name == "td" {
				for _, attr := range []string{"colspan", "rowspan"} {
					num, _ := strconv.Atoi(epubNodeAttr(node, attr))
					if num > 1 && num <= 32 {
						result.WriteString(" " + attr + "=\"" + strconv.Itoa(num) + "\"")
					}
				}
			}
			result.WriteByte('>')
		}
		if name == "img" || name == "br" || name == "hr" {
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if allowed {
			result.WriteString("</" + name + ">")
		}
	}
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		walk(child)
	}
	var heading string
	for _, tag := range []string{"h1", "h2", "h3"} {
		if n := findEPUBElement(body, tag); n != nil {
			heading = strings.TrimSpace(epubNodeText(n))
			if len(heading) > 120 {
				heading = string([]rune(heading)[:min(50, len([]rune(heading)))])
			}
			break
		}
	}
	return result.String(), heading, nil
}

func readEPUBNavigation(raw []byte, location string, chapters map[string]int) []epubTOCItem {
	reader, err := charset.NewReader(bytes.NewReader(raw), "")
	if err != nil {
		reader = bytes.NewReader(raw)
	}
	tree, err := nethtml.Parse(reader)
	if err != nil {
		return nil
	}
	var nav *nethtml.Node
	var find func(*nethtml.Node)
	find = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == "nav" {
			if nav == nil {
				nav = node
			}
			for _, attr := range node.Attr {
				if (attr.Key == "epub:type" || (attr.Namespace == "epub" && attr.Key == "type") || attr.Key == "type") && strings.Contains(" "+attr.Val+" ", " toc ") {
					nav = node
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(tree)
	if nav == nil {
		return nil
	}
	var out []epubTOCItem
	var visit func(*nethtml.Node, int)
	visit = func(node *nethtml.Node, depth int) {
		if len(out) >= 200 {
			return
		}
		if node.Type == nethtml.ElementNode && node.Data == "a" {
			href := epubNodeAttr(node, "href")
			target, frag, err := safeEPUBPath(location, href)
			if err == nil {
				if idx, ok := chapters[target]; ok {
					title := strings.TrimSpace(epubNodeText(node))
					if title != "" {
						out = append(out, epubTOCItem{Title: title, Chapter: idx, Anchor: frag, Depth: max(0, min(depth, 5))})
					}
				}
			}
			return
		}
		if node.Type == nethtml.ElementNode && node.Data == "ol" {
			depth++
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, depth)
		}
	}
	visit(nav, -1)
	return out
}

type epubNCX struct {
	Items []epubNavPoint `xml:"navMap>navPoint"`
}
type epubNavPoint struct {
	Label struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []epubNavPoint `xml:"navPoint"`
}

func readEPUBNCX(raw []byte, location string, chapters map[string]int) []epubTOCItem {
	var toc epubNCX
	if err := decodeEPUBXML(raw, &toc); err != nil {
		return nil
	}
	var items []epubTOCItem
	var visit func([]epubNavPoint, int)
	visit = func(points []epubNavPoint, depth int) {
		for _, point := range points {
			target, fragment, err := safeEPUBPath(location, point.Content.Src)
			if err == nil {
				if idx, ok := chapters[target]; ok {
					items = append(items, epubTOCItem{
						Title:   strings.TrimSpace(point.Label.Text),
						Chapter: idx, Anchor: fragment, Depth: max(0, min(depth, 5)),
					})
				}
			}
			if len(items) >= 200 {
				return
			}
			visit(point.Children, depth+1)
		}
	}
	visit(toc.Items, 0)
	return items
}
