package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	nethtml "golang.org/x/net/html"
)

// Fixed upstream distribution, embedded for offline reading. Its license and
// bundled dependency notices are retained in assets and the app's resources.
//
//go:embed assets/mermaid-12.3.0.min.js
var mermaidScript string

//go:embed markdown-preview.js
var markdownScript string

// Match familiar GitHub-style fragments, including non-Latin headings.
type headingIDs struct{ used map[string]bool }

func (ids *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	var slug strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(string(value))) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsMark(r), r == '-', r == '_':
			slug.WriteRune(r)
		case unicode.IsSpace(r):
			slug.WriteByte('-')
		}
	}
	base := slug.String()
	if base == "" {
		base = "section"
	}
	id := base
	for i := 1; ids.used[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	ids.used[id] = true
	return []byte(id)
}

func (ids *headingIDs) Put(value []byte) { ids.used[string(value)] = true }

// Parse Markdown structure so HTML-looking text inside a code fence does not
// switch renderers, while real anchors and nested Mermaid fences do.
func requiresHTMLRendering(content string) bool {
	if !strings.ContainsAny(content, "<`~[") {
		return false
	}
	source := []byte(content)
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(source))
	required := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.HTMLBlock, *ast.RawHTML:
			required = true
		case *ast.FencedCodeBlock:
			required = strings.EqualFold(string(node.Language(source)), "mermaid")
		case *ast.Link:
			dest := string(node.Destination)
			required = !strings.Contains(dest, ":") && !strings.HasPrefix(dest, "//")
		}
		if required {
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return required
}

// Reuse the exact tokenizer used by native source files and native Markdown.
// This runs after HTML parsing; token text is always escaped, never trusted HTML.
func emitHighlightedCode(out *strings.Builder, node *nethtml.Node) bool {
	language := ""
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			for _, class := range strings.Fields(attr.Val) {
				if strings.HasPrefix(class, "language-") {
					language = strings.ToLower(strings.TrimPrefix(class, "language-"))
				}
			}
		}
	}
	var source strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.TextNode {
			return false
		}
		source.WriteString(child.Data)
	}
	if language == "mermaid" {
		out.WriteString(`<code class="language-mermaid">` + html.EscapeString(source.String()) + `</code>`)
		return true
	}
	l := editor.LanguageNamed(language)
	if l == nil || l.Tokenizer == nil {
		return false
	}
	out.WriteString(`<code class="language-` + html.EscapeString(language) + `">`)
	var state uint64
	var tokens []editor.Token
	for _, line := range strings.SplitAfter(source.String(), "\n") {
		body := strings.TrimSuffix(line, "\n")
		tokens, state = l.Tokenizer.Tokenize(body, state, tokens[:0])
		start := 0
		for _, token := range tokens {
			end := min(len(body), max(start, token.End))
			fmt.Fprintf(out, `<span class="cv-token-%d">%s</span>`, token.Kind, html.EscapeString(body[start:end]))
			start = end
		}
		out.WriteString(html.EscapeString(body[start:]))
		if strings.HasSuffix(line, "\n") {
			out.WriteByte('\n')
		}
	}
	out.WriteString(`</code>`)
	return true
}

var codeTokenCSS = func() string {
	var css strings.Builder
	for _, dark := range []bool{false, true} {
		theme, mode := ui.LightTheme(), "light"
		if dark {
			theme, mode = ui.DarkTheme(), "dark"
		}
		var vars strings.Builder
		for i, token := range editor.DefaultTheme(theme).Tokens {
			if token.Color.A != 0 {
				fmt.Fprintf(&vars, "--code-%d:#%02x%02x%02x;", i, token.Color.R, token.Color.G, token.Color.B)
			}
		}
		if dark {
			css.WriteString("@media(prefers-color-scheme:dark){:root:not([data-theme=light]){" + vars.String() + "}}")
		} else {
			css.WriteString(":root{" + vars.String() + "}")
		}
		css.WriteString(":root[data-theme=" + mode + "]{" + vars.String() + "}")
	}
	for i, token := range editor.DefaultTheme(ui.LightTheme()).Tokens {
		fmt.Fprintf(&css, ".cv-token-%d{color:var(--code-%d,var(--ink));", i, i)
		if token.Bold {
			css.WriteString("font-weight:600;")
		}
		if token.Italic {
			css.WriteString("font-style:italic;")
		}
		css.WriteString("}")
	}
	return css.String()
}()

func (a *application) markdownScripts(body, fragment string) (string, string, error) {
	hasDiagrams := strings.Contains(body, `class="language-mermaid"`)
	if !hasDiagrams && fragment == "" {
		return "", "", nil
	}
	key := make([]byte, 18)
	if _, err := rand.Read(key); err != nil {
		return "", "", err
	}
	nonce := base64.RawStdEncoding.EncodeToString(key)
	config, err := json.Marshal(map[string]string{
		"fragment": fragment,
		"error":    a.tr("Diagram could not be rendered. The source is shown below."),
	})
	if err != nil {
		return "", "", err
	}
	script := `<script nonce="` + nonce + `">`
	if hasDiagrams {
		// Prevent a literal closing tag in the bundled JS from ending this element.
		script += strings.ReplaceAll(mermaidScript, "</script", `<\/script`) + "\n"
	}
	script += "const catviewPreview=" + string(config) + ";\n" + markdownScript + `</script>`
	return script, nonce, nil
}
