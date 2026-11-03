package main

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *application) documentPane(c *ui.Context) {
	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		if len(a.tabs) > 0 {
			a.tabBar(c)
		}
		a.reader(c)
	})
}

func (a *application) tabBar(c *ui.Context) {
	t := c.Theme()
	var closing *document
	ui.ScrollHorizontal(c).Height(36).FillWidth().Background(t.Surface).Children(func() {
		tabs := ui.TabsBase(c, &a.activeTab, len(a.tabs))
		tabs.List.Gap(0).Children(func() {
			for i, d := range a.tabs {
				tab := tabs.Tab(i).Width(190).Height(36).Padding(4, 10).Label(filepath.Base(d.path)).Tooltip(d.path)
				if d == a.doc {
					tab.Background(t.Background)
				}
				tab.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Gap(6).Children(func() {
						name := filepath.Base(d.path)
						if documentModified(d) {
							name = "• " + name
						}
						ui.Text(c, name).SingleLine().Grow(1)
						if ui.Button(c, "×").Size(22, 22).Label(a.tr("Close tab") + " " + filepath.Base(d.path)).Tooltip(a.tr("Close tab")).Clicked() {
							closing = d
						}
					})
				})
			}
		})
		if tabs.List.Changed() && a.activeTab >= 0 && a.activeTab < len(a.tabs) {
			a.loadSerial++
			a.activateDocument(a.tabs[a.activeTab])
		}
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
		switch {
		case d.kind == kindPDF:
			a.openPDF(d.path)
		case d.kind == kindEPUB:
			a.openEPUB(d)
		case d.kind == kindHTML || d.kind == kindOffice || (d.kind == kindMarkdown && d.htmlRich):
			a.openHTMLPreview(d)
		}
	}
	a.restoreSelectedFileRow()
	a.installMenu()
	a.win.Invalidate()
}

func (a *application) removeTab(d *document) {
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
	if d == nil || a.closePrompt {
		return
	}
	if d.saving {
		a.status = a.tr("Cannot close while saving. Please try again.")
		return
	}
	if !documentModified(d) {
		a.removeTab(d)
		if closeWindow {
			a.win.Close()
		}
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
				return
			}
			if res.Button == 2 {
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
				return
			}
			a.removeTab(d)
			if closeWindow {
				win.Close()
			}
		})
	}()
}
