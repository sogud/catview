package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

// Use the standard parsers and the existing editor's folding/highlighting.
// Formatting is a read-only projection; editing always uses the original bytes.
func formattedText(source, path string) *editor.State {
	if len(source) > 2<<20 {
		return nil
	}
	var out bytes.Buffer
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if json.Indent(&out, []byte(source), "", "  ") != nil {
			return nil
		}
	case ".xml", ".plist":
		d := xml.NewDecoder(strings.NewReader(source))
		e := xml.NewEncoder(&out)
		e.Indent("", "  ")
		depth, count := 0, 0
		for {
			token, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil
			}
			count++
			switch token.(type) {
			case xml.StartElement:
				depth++
			case xml.EndElement:
				depth--
			}
			if depth > 80 || count > 100000 || out.Len() > 4<<20 {
				return nil
			}
			if e.EncodeToken(token) != nil {
				return nil
			}
		}
		if e.Flush() != nil {
			return nil
		}
	default:
		return nil
	}
	b := editor.NewBuffer(out.String())
	b.SetLanguage(editor.LanguageFor(path))
	return &editor.State{Buffer: b, ReadOnly: true, NoActiveLine: true, LineNumbers: editor.LineNumbersOff}
}

func (d *document) visibleCode() *editor.State {
	if d.formatted != nil && !d.showSource && !d.editing {
		return d.formatted
	}
	return d.code
}
