package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	officeMaxZipFiles = 5000
	officeMaxUnpacked = 150 << 20
	officeMaxXML      = 14 << 20
	officeMaxImage    = 8 << 20
	officeMaxSlides   = 300
)

type officeNode struct {
	name     string
	text     string
	attrs    []xml.Attr
	children []*officeNode
}

func (n *officeNode) attr(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
func (n *officeNode) relID() string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Local == "embed" || a.Name.Local == "id" && strings.Contains(a.Name.Space, "relationships") {
			return a.Value
		}
	}
	return ""
}
func (n *officeNode) child(name string) *officeNode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}
func (n *officeNode) desc(name string) *officeNode {
	if n == nil {
		return nil
	}
	if n.name == name {
		return n
	}
	for _, c := range n.children {
		if found := c.desc(name); found != nil {
			return found
		}
	}
	return nil
}
func (n *officeNode) all(name string) []*officeNode {
	var out []*officeNode
	var visit func(*officeNode)
	visit = func(v *officeNode) {
		if v == nil {
			return
		}
		if v.name == name {
			out = append(out, v)
		}
		for _, c := range v.children {
			visit(c)
		}
	}
	visit(n)
	return out
}

// parseOfficeNode uses one small DOM for the XML part the user opened.
func parseOfficeNode(data []byte) (*officeNode, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	root := &officeNode{name: "root"}
	stack := []*officeNode{root}
	var count int
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if len(stack) > 85 || count > 250000 {
				return nil, errors.New("Office XML 太复杂")
			}
			n := &officeNode{name: t.Name.Local, attrs: t.Attr}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
			stack = append(stack, n)
			count++
		case xml.EndElement:
			if len(stack) < 2 {
				return nil, errors.New("Office XML 标记不匹配")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			top := stack[len(stack)-1]
			top.text += string(t)
		}
	}
	if len(stack) != 1 {
		return nil, errors.New("Office XML 未闭合")
	}
	return root, nil
}

type officeArchive struct {
	zipReader  *zip.ReadCloser
	files      map[string]*zip.File
	images     map[string]string
	imageBytes int64
}

func openOfficeArchive(filename string) (*officeArchive, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("Office 文档不是有效的 ZIP 文件：%w", err)
	}
	files := make(map[string]*zip.File)
	var total uint64
	if len(archive.File) > officeMaxZipFiles {
		archive.Close()
		return nil, errors.New("Office 文件数过多")
	}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := file.Name
		if path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") ||
			file.FileInfo().Mode()&os.ModeSymlink != 0 {
			archive.Close()
			return nil, errors.New("Office 文件路径不安全")
		}
		total += file.UncompressedSize64
		if total > officeMaxUnpacked {
			archive.Close()
			return nil, errors.New("Office 文档解压体积超过限制")
		}
		files[name] = file
	}
	return &officeArchive{zipReader: archive, files: files, images: make(map[string]string)}, nil
}

func (a *officeArchive) read(name string, limit int64) ([]byte, error) {
	file := a.files[name]
	if file == nil {
		return nil, fmt.Errorf("Office 文件缺失：%s", name)
	}
	if file.UncompressedSize64 > uint64(limit) {
		return nil, errors.New("Office 资源过大")
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	buf, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > limit {
		return nil, errors.New("Office 资源超过限制")
	}
	return buf, nil
}

func (a *officeArchive) xml(name string) (*officeNode, error) {
	data, err := a.read(name, officeMaxXML)
	if err != nil {
		return nil, err
	}
	return parseOfficeNode(data)
}

func (a *officeArchive) relationships(source string) map[string]string {
	dir := path.Dir(source)
	relsPath := path.Join(dir, "_rels", path.Base(source)+".rels")
	result := make(map[string]string)
	root, err := a.xml(relsPath)
	if err != nil {
		return result
	}
	for _, item := range root.all("Relationship") {
		if item.attr("TargetMode") == "External" {
			continue
		}
		target := item.attr("Target")
		if strings.ContainsAny(target, "\x00\\") {
			continue
		}
		var resolved string
		if strings.HasPrefix(target, "/") {
			resolved = path.Clean(strings.TrimPrefix(target, "/"))
		} else {
			resolved = path.Clean(path.Join(dir, target))
		}
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			continue
		}
		if a.files[resolved] != nil {
			result[item.attr("Id")] = resolved
		}
	}
	return result
}

func (a *officeArchive) imageData(filename string) string {
	if cached, ok := a.images[filename]; ok {
		return cached
	}
	var mime string
	switch strings.ToLower(path.Ext(filename)) {
	case ".png":
		mime = "image/png"
	case ".jpeg", ".jpg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	default:
		return ""
	}
	data, err := a.read(filename, officeMaxImage)
	if err != nil {
		return ""
	}
	if a.imageBytes+int64(len(data)) > 28<<20 {
		return ""
	}
	a.imageBytes += int64(len(data))
	src := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	a.images[filename] = src
	return src
}

func escapeOfficeText(t string) string {
	return stdhtml.EscapeString(t)
}
func (a *officeArchive) wordRun(n *officeNode, relations map[string]string) string {
	var b strings.Builder
	var visit func(*officeNode)
	visit = func(item *officeNode) {
		if item == nil {
			return
		}
		switch item.name {
		case "t":
			b.WriteString(escapeOfficeText(item.text))
			return
		case "br":
			b.WriteString("<br>")
			return
		case "tab":
			b.WriteString("&emsp;")
			return
		case "blip":
			src := a.imageData(relations[item.relID()])
			if src != "" {
				b.WriteString(`<img src="` + src + `" style="display:block;max-width:100%;margin:.8em auto" alt="文档图片">`)
			}
			return
		case "rPr", "pPr", "drawingPr":
			return
		}
		for _, ch := range item.children {
			visit(ch)
		}
	}
	visit(n)
	return b.String()
}

func (a *officeArchive) wordParagraph(n *officeNode, rels map[string]string) string {
	pr := n.child("pPr")
	tag := "p"
	style := ""
	if pr != nil {
		if st := pr.child("pStyle"); st != nil {
			val := strings.ToLower(st.attr("val"))
			switch {
			case strings.Contains(val, "heading1") || val == "title":
				tag = "h1"
			case strings.Contains(val, "heading2"):
				tag = "h2"
			case strings.Contains(val, "heading3"):
				tag = "h3"
			case strings.Contains(val, "heading4"):
				tag = "h4"
			}
		}
		if jc := pr.child("jc"); jc != nil && (jc.attr("val") == "center" || jc.attr("val") == "right") {
			style = ` style="text-align:` + jc.attr("val") + `"`
		}
	}
	var body strings.Builder
	for _, part := range n.children {
		if part.name == "pPr" {
			continue
		}
		if part.name == "r" {
			text := a.wordRun(part, rels)
			if text == "" {
				continue
			}
			props := part.child("rPr")
			bold, italic, underline := false, false, false
			if props != nil {
				bold = props.child("b") != nil
				italic = props.child("i") != nil
				underline = props.child("u") != nil
			}
			if bold {
				body.WriteString("<strong>")
			}
			if italic {
				body.WriteString("<em>")
			}
			if underline {
				body.WriteString("<u>")
			}
			body.WriteString(text)
			if underline {
				body.WriteString("</u>")
			}
			if italic {
				body.WriteString("</em>")
			}
			if bold {
				body.WriteString("</strong>")
			}
		} else {
			body.WriteString(a.wordRun(part, rels))
		}
	}
	if body.Len() == 0 {
		return ""
	}
	return "<" + tag + style + ">" + body.String() + "</" + tag + ">"
}

func (a *officeArchive) wordTable(table *officeNode, rels map[string]string) string {
	var b strings.Builder
	b.WriteString("<table><tbody>")
	for _, row := range table.children {
		if row.name != "tr" {
			continue
		}
		b.WriteString("<tr>")
		for _, cell := range row.children {
			if cell.name != "tc" {
				continue
			}
			b.WriteString("<td>")
			for _, part := range cell.children {
				switch part.name {
				case "p":
					b.WriteString(a.wordParagraph(part, rels))
				case "tbl":
					b.WriteString(a.wordTable(part, rels))
				}
			}
			b.WriteString("</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func (a *officeArchive) docxHTML() (string, error) {
	doc, err := a.xml("word/document.xml")
	if err != nil {
		return "", err
	}
	body := doc.desc("body")
	if body == nil {
		return "", errors.New("Word 正文不存在")
	}
	rels := a.relationships("word/document.xml")
	var b strings.Builder
	for _, part := range body.children {
		if b.Len() > 32<<20 {
			return "", errors.New("Word 预览内容过大")
		}
		switch part.name {
		case "p":
			b.WriteString(a.wordParagraph(part, rels))
		case "tbl":
			b.WriteString(a.wordTable(part, rels))
		}
	}
	if b.Len() == 0 {
		return `<p style="text-align:center">文档内容为空</p>`, nil
	}
	return b.String(), nil
}

func parseOfficeInt(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func shapeCSS(node *officeNode, sw, sh int64) string {
	xf := node.desc("xfrm")
	if xf == nil {
		return ""
	}
	off, ext := xf.child("off"), xf.child("ext")
	if off == nil || ext == nil {
		return ""
	}
	x := parseOfficeInt(off.attr("x"), 0)
	y := parseOfficeInt(off.attr("y"), 0)
	w := parseOfficeInt(ext.attr("cx"), 0)
	h := parseOfficeInt(ext.attr("cy"), 0)
	if w <= 0 || h <= 0 {
		return ""
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= sw || y >= sh {
		return ""
	}
	return fmt.Sprintf("left:%.2f%%;top:%.2f%%;width:%.2f%%;height:%.2f%%",
		float64(x)*100/float64(sw), float64(y)*100/float64(sh),
		float64(w)*100/float64(sw), float64(h)*100/float64(sh))
}

func pptParagraph(node *officeNode) string {
	var b strings.Builder
	for _, run := range node.children {
		if run.name == "r" || run.name == "fld" {
			t := run.desc("t")
			if t == nil {
				continue
			}
			content := escapeOfficeText(t.text)
			if pr := run.child("rPr"); pr != nil {
				if pr.attr("b") == "1" {
					content = "<strong>" + content + "</strong>"
				}
				if pr.attr("i") == "1" {
					content = "<em>" + content + "</em>"
				}
			}
			b.WriteString(content)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "<p>" + b.String() + "</p>"
}

func pptShapeText(node *officeNode) string {
	tx := node.child("txBody")
	if tx == nil {
		tx = node.desc("txBody")
	}
	if tx == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range tx.children {
		if p.name == "p" {
			b.WriteString(pptParagraph(p))
		}
	}
	return b.String()
}

func (a *officeArchive) pptTable(node *officeNode) string {
	tbl := node.desc("tbl")
	if tbl == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<table><tbody>")
	for _, r := range tbl.children {
		if r.name != "tr" {
			continue
		}
		b.WriteString("<tr>")
		for _, cell := range r.children {
			if cell.name != "tc" {
				continue
			}
			b.WriteString("<td>")
			for _, p := range cell.all("p") {
				b.WriteString(pptParagraph(p))
			}
			b.WriteString("</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func pptSlideItems(node *officeNode) []*officeNode {
	var out []*officeNode
	var walk func(*officeNode)
	walk = func(n *officeNode) {
		if n == nil {
			return
		}
		if n.name == "sp" || n.name == "pic" || n.name == "graphicFrame" {
			out = append(out, n)
			return
		}
		for _, ch := range n.children {
			walk(ch)
		}
	}
	walk(node.desc("spTree"))
	return out
}

func (a *officeArchive) slideHTML(filename string, sw, sh int64) (string, error) {
	root, err := a.xml(filename)
	if err != nil {
		return "", err
	}
	rels := a.relationships(filename)
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<section class="cv-slide" style="--slide-aspect:%d/%d">`, sw, sh))
	var fallbacks []string
	for _, item := range pptSlideItems(root) {
		kind := item.name
		body := ""
		switch kind {
		case "sp":
			body = pptShapeText(item)
		case "graphicFrame":
			body = a.pptTable(item)
		case "pic":
			blip := item.desc("blip")
			if blip != nil {
				src := a.imageData(rels[blip.relID()])
				if src != "" {
					body = `<img src="` + src + `" alt="幻灯片图片" style="width:100%;height:100%;object-fit:contain">`
				}
			}
		}
		if body == "" {
			continue
		}
		pos := shapeCSS(item, sw, sh)
		if pos == "" {
			fallbacks = append(fallbacks, body)
			continue
		}
		var styling string
		if kind == "sp" {
			styling = "font-size:1.05em;padding:.2em .3em"
		}
		b.WriteString(`<div class="cv-shape" style="` + pos + ";" + styling + `">` + body + "</div>")
	}
	if len(fallbacks) > 0 {
		b.WriteString(`<div style="position:absolute;left:6%;top:12%;width:88%;height:80%;overflow:auto">`)
		b.WriteString(strings.Join(fallbacks, ""))
		b.WriteString("</div>")
	}
	b.WriteString("</section>")
	return b.String(), nil
}

func (a *officeArchive) pptxHTML() (string, error) {
	root, err := a.xml("ppt/presentation.xml")
	if err != nil {
		return "", err
	}
	sw, sh := int64(12192000), int64(6858000)
	if sz := root.desc("sldSz"); sz != nil {
		sw = parseOfficeInt(sz.attr("cx"), sw)
		sh = parseOfficeInt(sz.attr("cy"), sh)
	}
	relations := a.relationships("ppt/presentation.xml")
	var slides []string
	for _, id := range root.all("sldId") {
		target := relations[id.relID()]
		if target != "" && strings.HasPrefix(target, "ppt/slides/slide") && strings.HasSuffix(target, ".xml") {
			slides = append(slides, target)
		}
	}
	if len(slides) == 0 {
		for file := range a.files {
			if strings.HasPrefix(file, "ppt/slides/slide") && strings.HasSuffix(file, ".xml") && strings.Count(file, "/") == 2 {
				slides = append(slides, file)
			}
		}
		sort.Slice(slides, func(i, j int) bool {
			return slideNumber(slides[i]) < slideNumber(slides[j])
		})
	}
	if len(slides) == 0 {
		return "", errors.New("没有找到 PowerPoint 幻灯片")
	}
	if len(slides) > officeMaxSlides {
		return "", errors.New("幻灯片超过 300 页限制")
	}
	var b strings.Builder
	for i, file := range slides {
		if b.Len() > 64<<20 {
			return "", errors.New("PowerPoint 预览超过大小限制")
		}
		page, err := a.slideHTML(file, sw, sh)
		if err != nil {
			return "", err
		}
		b.WriteString(`<div class="cv-slide-title">` + strconv.Itoa(i+1) + ` / ` + strconv.Itoa(len(slides)) + `</div>`)
		b.WriteString(page)
	}
	return b.String(), nil
}

func slideNumber(p string) int {
	s := strings.TrimSuffix(strings.TrimPrefix(path.Base(p), "slide"), ".xml")
	n, _ := strconv.Atoi(s)
	return n
}

func convertLegacyDOC(filename string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/textutil", "-convert", "html", "-stdout", "-noload", "-nostore", "--", filename)
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("旧版 Word 无法转换：%w", err)
	}
	if len(data) > 24<<20 {
		return "", errors.New("Word 输出超过预览限制")
	}
	return string(data), nil
}

func quickLookLegacyPPT(filename string) (string, error) {
	dir, err := os.MkdirTemp("", "catview-ppt-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/qlmanage", "-t", "-s", "1200", "-o", dir, filename)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("旧版 PPT 缩略图不可用：%w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, file := range entries {
		if strings.HasSuffix(strings.ToLower(file.Name()), ".png") {
			data, err := os.ReadFile(filepath.Join(dir, file.Name()))
			if err != nil {
				return "", err
			}
			if len(data) > officeMaxImage {
				return "", errors.New("PPT 缩略图过大")
			}
			src := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
			return `<figure><img src="` + src + `" alt="PowerPoint 首页预览"><figcaption>旧版 PPT：当前仅提供第一页缩略图，完整多页预览需要转换为 PPTX。</figcaption></figure>`, nil
		}
	}
	return "", errors.New("旧版 PPT 没有可用的预览图")
}

// readOfficePreview never modifies the source file.
func readOfficePreview(filename string) (html string, presentation bool, legacy bool, err error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".doc":
		html, err = convertLegacyDOC(filename)
		return html, false, true, err
	case ".ppt":
		html, err = quickLookLegacyPPT(filename)
		return html, true, false, err
	case ".docx", ".pptx":
		archive, er := openOfficeArchive(filename)
		if er != nil {
			return "", false, false, er
		}
		defer archive.zipReader.Close()
		if strings.EqualFold(filepath.Ext(filename), ".docx") {
			html, err = archive.docxHTML()
			return html, false, false, err
		}
		html, err = archive.pptxHTML()
		return html, true, false, err
	default:
		return "", false, false, errors.New("不支持此 Office 格式")
	}
}
