package main

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var closeTabGlyph = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m3 3 6 6M9 3 3 9"/></svg>`))

func (a *application) documentPane(c *ui.Context) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(c.Theme().Background).Children(func() {
		if len(a.tabs) > 0 {
			a.tabBar(c)
		} else if !a.sidebarVisible {
			a.toolbar(c)
		}
		a.reader(c)
	})
}

func (a *application) tabBar(c *ui.Context) {
	original := c.Theme()
	t := windowChromeTheme(original.Dark)
	c.SetTheme(t)
	defer c.SetTheme(original)
	var closing *document
	height := max(float32(32), c.TitleBar().Height)
	ui.Row(c).Height(height).FillWidth().DragWindow().Background(t.Background).Children(func() {
		if !a.sidebarVisible {
			ui.Row(c).Width(c.TitleBar().Left+46).Height(height).Padding(0, 4, 0, c.TitleBar().Left+8).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "").Size(28, 28).Padding(0).Center().Label(a.tr("Show sidebar")).Children(func() { ui.Icon(c, sidebarGlyph).FontSize(18) }).Clicked() {
					a.toggleSidebar()
				}
			})
		}
		ui.ScrollHorizontal(c).Height(height).Grow(1).Children(func() {
			tabs := ui.TabsBase(c, &a.activeTab, len(a.tabs))
			tabs.List.Gap(0).Children(func() {
				for i, d := range a.tabs {
					tab := ui.Row(c).Key(d.path).Width(190).Height(height).Padding(0, 8).Gap(4).AlignItems(ui.Center)
					if d == a.doc {
						tab.Background(t.Background)
					} else {
						tab.Background(t.Surface)
					}
					tab.ContextMenu(func(m *ui.Menu) { a.tabContextMenu(m, d) })
					tab.Children(func() {
						tabs.Tab(i).Grow(1).MinWidth(0).Height(height).Padding(0, 2).AlignItems(ui.Center).Label(filepath.Base(d.path)).Tooltip(d.path).Children(func() {
							name := filepath.Base(d.path)
							if documentModified(d) {
								name = "• " + name
							}
							ui.Text(c, name).SingleLine().Grow(1).MinWidth(0)
						})
						// A sibling button remains independently accessible and cannot
						// select a tab when the user only means to close it.
						close := ui.ButtonBase(c).Size(22, 22).Padding(0).Center().Radius(4).
							Label(a.tr("Close tab") + " " + filepath.Base(d.path)).Tooltip(a.tr("Close tab"))
						if close.Hovered() {
							close.Background(t.SurfaceHover)
						}
						close.Children(func() { ui.Icon(c, closeTabGlyph).FontSize(12) })
						if close.Clicked() {
							closing = d
						}
					})
				}
			})
			if tabs.List.Changed() && a.activeTab >= 0 && a.activeTab < len(a.tabs) {
				a.loadSerial++
				a.activateDocument(a.tabs[a.activeTab])
			}
		})
	})
	if closing != nil {
		a.requestCloseTab(closing, false)
	}
}

func documentModified(d *document) bool {
	return d != nil && d.code != nil && d.code.Buffer.Modified()
}

func (a *application) activateDocument(d *document) {
	for i, tab := range a.tabs {
		if tab == d {
			a.activeTab = i
			break
		}
	}
	if a.doc == d {
		return
	}
	a.closeEPUB()
	a.closeHTMLPreview()
	a.closeNativePreview()
	if a.pdf != nil {
		a.pdf.Destroy()
		a.pdf = nil
	}
	a.doc = d
	a.selected = ""
	if d == nil {
		a.win.SetTitle("CatView")
		a.win.SetRepresentedFile("")
	} else {
		a.selected = d.path
		a.status = "已打开 " + filepath.Base(d.path)
		a.win.SetTitle(filepath.Base(d.path))
		a.win.SetRepresentedFile(d.path)
		a.openDocumentPreview(d)
	}
	a.restoreSelectedFileRow()
	a.installMenu()
	a.win.Invalidate()
}

func (a *application) removeTab(d *document) {
	if d.loadCancel != nil {
		d.loadCancel()
	}
	cancelTranslation(d)
	for i, tab := range a.tabs {
		if tab != d {
			continue
		}
		a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
		if len(a.tabs) == 0 {
			a.loadEpoch++ // Closing all documents also cancels pending opens.
		}
		if a.doc == d {
			a.loadSerial++
			var next *document
			if len(a.tabs) > 0 {
				next = a.tabs[min(i, len(a.tabs)-1)]
			}
			a.activateDocument(next)
		} else {
			for index, current := range a.tabs {
				if current == a.doc {
					a.activeTab = index
					break
				}
			}
		}
		a.win.Invalidate()
		return
	}
}

func (a *application) requestCloseTab(d *document, closeWindow bool) {
	if a.closeBatch {
		return
	}
	a.closeTabThen(d, func(closed bool) {
		if closed && closeWindow {
			a.win.Close()
		}
	})
}

func (a *application) closeTabThen(d *document, done func(bool)) {
	if d == nil || a.closePrompt || a.fileOperation {
		done(false)
		return
	}
	if d.saving {
		a.status = a.tr("Cannot close while saving. Please try again.")
		done(false)
		return
	}
	if !documentModified(d) {
		a.removeTab(d)
		done(true)
		return
	}
	a.closePrompt = true
	a.loadSerial++
	d.saving = true
	a.activateDocument(d)
	win := a.win
	content := d.code.Buffer.String()
	version := d.code.Buffer.Version()
	digest, mode := d.digest, d.mode
	tr := a.translatorForUI()
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: win, Type: mygo.MessageWarning,
			Message: fmt.Sprintf(tr("Save changes to %s?"), filepath.Base(d.path)),
			Buttons: []string{tr("Save"), tr("Don’t Save"), tr("Cancel")}, DefaultButton: 0, CancelButton: 2,
		})
		newDigest := digest
		if err == nil && res.Button == 0 {
			newDigest, err = safeSaveText(d.path, digest, mode, content)
		}
		win.Update(func() {
			a.closePrompt = false
			d.saving = false
			if err != nil {
				a.status = a.tr("Cannot close: ") + a.tr(err.Error())
				done(false)
				return
			}
			if res.Button == 2 {
				done(false)
				return
			}
			if res.Button == 0 {
				d.digest = newDigest
				d.source = content
				if d.kind == kindMarkdown {
					d.htmlRich = requiresHTMLRendering(content)
				}
			}
			// A newer edit must never disappear after the save finishes.
			if res.Button == 0 && d.code.Buffer.Version() != version {
				a.status = a.tr("Cannot close: new edits appeared while saving. Save again.")
				done(false)
				return
			}
			a.removeTab(d)
			done(true)
		})
	}()
}

func (a *application) tabContextMenu(m *ui.Menu, target *document) {
	index := -1
	for i, d := range a.tabs {
		if d == target {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	disabled := a.fileOperation || a.closePrompt || a.closeBatch
	if m.Item(a.tr("Close tab")).Disabled(disabled).Chosen() {
		a.requestCloseTab(target, false)
	}
	for _, op := range []struct {
		label       string
		mode        int
		unavailable bool
	}{
		{"Close other tabs", 0, len(a.tabs) < 2}, {"Close tabs to the left", 1, index == 0}, {"Close tabs to the right", 2, index == len(a.tabs)-1}, {"Close all tabs", 3, false},
	} {
		if m.Item(a.tr(op.label)).Disabled(disabled || op.unavailable).Chosen() {
			var closing []*document
			for i, d := range a.tabs {
				if op.mode == 3 || op.mode == 0 && d != target || op.mode == 1 && i < index || op.mode == 2 && i > index {
					closing = append(closing, d)
				}
			}
			a.closeTabs(closing)
		}
	}
	m.Separator()
	if m.Item(a.tr("Copy path")).Chosen() {
		mygo.Clipboard.WriteText(target.path)
	}
	if m.Item(a.tr("Show in Finder")).Chosen() {
		mygo.Shell.ShowItemInFolder(target.path)
	}
}

func (a *application) closeTabs(documents []*document) {
	if a.closeBatch || a.closePrompt || a.fileOperation {
		return
	}
	a.closeBatch = true
	var next func(bool)
	next = func(proceed bool) {
		if !proceed || len(documents) == 0 || a.win.IsDestroyed() {
			a.closeBatch = false
			return
		}
		d := documents[0]
		documents = documents[1:]
		a.closeTabThen(d, next)
	}
	next(true)
}
