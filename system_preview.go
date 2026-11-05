package main

import (
	"path/filepath"
	"strings"
)

// Offer the OS viewer explicitly for document types whose layout support
// varies by macOS and installed Quick Look extensions. This is not a claim
// that every system has a renderer for every candidate.
func canUseSystemPreview(d *document) bool {
	if d == nil || d.loading || d.kind == kindSystem || d.editing || documentModified(d) {
		return false
	}
	switch strings.ToLower(filepath.Ext(d.path)) {
	case ".doc", ".docx", ".docm", ".ppt", ".pptx", ".pptm", ".xls", ".xlsx", ".xlsm", ".xlsb", ".odt", ".ods", ".odp", ".rtf", ".pages", ".numbers", ".key", ".pdf":
		return true
	}
	return false
}

func (a *application) toggleSystemPreview() {
	d := a.doc
	if !canUseSystemPreview(d) {
		return
	}
	a.closeNativePreview()
	a.closeHTMLPreview()
	if a.pdf != nil {
		a.pdf.Destroy()
		a.pdf = nil
	}
	d.systemPreview = !d.systemPreview
	d.previewError = ""
	if d.systemPreview {
		a.openNativePreview(d)
	} else {
		a.loadDocument(d)
	}
}

func (a *application) togglePreviewSource() {
	d := a.doc
	if d == nil || d.systemPreview || d.loading || d.editing || d.code == nil || !(d.kind == kindTable || d.kind == kindDocument || d.formatted != nil) {
		return
	}
	d.showSource = !d.showSource
	if d.kind == kindDocument {
		a.closeHTMLPreview()
		if !d.showSource {
			a.openHTMLPreview(d)
		}
	}
}
