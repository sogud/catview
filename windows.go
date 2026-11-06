package main

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Reader state belongs to a window. Only history and the updater are shared.
var readerWindows []*application
var activeReader *application
var updateOwner *application

func newReaderWindow(initialPath string, restore bool) *application {
	if updateOwner != nil && updateOwner.updates.phase == "installing" {
		return activeReader
	}
	a := newApplication()
	a.loadPreferences()
	loadOpenHistory(a.recentFolders)
	a.applySystemAppearance()
	a.resolveLanguage()
	a.loadTranslationSettings()
	if updateOwner != nil {
		a.updates = updateOwner.updates
	}
	stateKey := ""
	if len(readerWindows) == 0 {
		stateKey = "main"
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "CatView", Width: 1120, Height: 750, MinWidth: 680, MinHeight: 440,
		TitleBarStyle: mygo.TitleBarHidden, Vibrancy: a.sidebarMaterial(),
		StateKey: stateKey, Content: ui.View(a.view),
	})
	readerWindows = append(readerWindows, a)
	activeReader = a
	a.win.OnFocus(func() { activeReader = a; a.installMenu() })
	a.guardClose()
	a.startFileRefresh()
	if updateOwner == nil {
		updateOwner = a
		a.startUpdates()
	}
	a.win.OnClosed(func() {
		a.fileRefreshCancel()
		for _, d := range a.tabs {
			if d.loadCancel != nil {
				d.loadCancel()
			}
			cancelTranslation(d)
		}
		a.closeNativePreview()
		a.closeEPUB()
		a.closeHTMLPreview()
		if a.pdf != nil {
			a.pdf.Destroy()
			a.pdf = nil
		}
		if a.settings != nil {
			a.settings.Close()
		}
		if a.historyWindow != nil {
			a.historyWindow.Close()
		}
		remaining := readerWindows[:0]
		for _, other := range readerWindows {
			if other != a {
				remaining = append(remaining, other)
			}
		}
		readerWindows = remaining
		if activeReader == a {
			activeReader = nil
			if len(remaining) > 0 {
				activeReader = remaining[len(remaining)-1]
			}
		}
		if updateOwner == a {
			a.stopUpdates()
			updateOwner = nil
			if len(remaining) > 0 {
				updateOwner = remaining[0]
				shared := &updateController{}
				for _, other := range remaining {
					other.updates = shared
				}
				updateOwner.startUpdates()
			}
		}
		if activeReader != nil {
			activeReader.installMenu()
		}
	})
	if restore && initialPath == "" {
		folders := recentHistoryFolders()
		if len(folders) > 0 {
			initialPath = folders[0]
		}
	}
	if initialPath != "" {
		if path, err := filepath.Abs(initialPath); err == nil {
			if info, err := os.Stat(path); err == nil {
				a.setRoot(path, info.IsDir())
			}
		}
	}
	a.installMenu()
	a.win.Focus()
	return a
}

func (a *application) chooseFolderInNewWindow() {
	if a.fileOperation {
		return
	}
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: a.tr("Choose a folder"), Directory: true})
		win.Update(func() {
			if err != nil {
				a.status = a.tr("Cannot open: ") + err.Error()
				return
			}
			if len(paths) > 0 {
				newReaderWindow(paths[0], false)
			}
		})
	}()
}

// Reading preferences are shared; folder, tabs and split position stay local.
func (a *application) shareReadingPreferences() {
	for _, other := range readerWindows {
		if other == a {
			continue
		}
		if other.markdownCodeMode == a.markdownCodeMode && other.readingFont == a.readingFont && other.codeFont == a.codeFont && other.language == a.language && other.colorTheme == a.colorTheme && other.themeMode == a.themeMode && other.accentIndex == a.accentIndex && other.fileIconStyle == a.fileIconStyle && other.fontSize == a.fontSize && other.lineHeight == a.lineHeight && other.readingWidth == a.readingWidth {
			continue
		}
		other.readingFont, other.codeFont = a.readingFont, a.codeFont
		other.markdownCodeMode = a.markdownCodeMode
		other.language, other.colorTheme, other.themeMode = a.language, a.colorTheme, a.themeMode
		other.accentIndex, other.fileIconStyle = a.accentIndex, a.fileIconStyle
		other.fontSize, other.lineHeight, other.readingWidth = a.fontSize, a.lineHeight, a.readingWidth
		other.resolveLanguage()
		other.refreshAppearance()
		other.installMenu()
		if other.settings != nil {
			other.settings.Invalidate()
		}
	}
}
