//go:build !darwin

package main

import "github.com/egoist/mygo/ui"

type nativePreview struct{}

func (a *application) openNativePreview(d *document) {
	d.previewError = "System preview requires macOS."
}
func (a *application) nativePreviewView(c *ui.Context) {}
func (a *application) closeNativePreview()             {}
