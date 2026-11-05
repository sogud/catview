package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

type savedReaderWindow struct {
	Root           string           `json:"root"`
	Tabs           []savedReaderTab `json:"tabs"`
	Active         string           `json:"active,omitempty"`
	SidebarVisible bool             `json:"sidebar_visible"`
	SidebarWidth   float32          `json:"sidebar_width"`
}
type savedReaderTab struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
}

func sessionPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// Save paths before quit closes any windows. Unsaved text is handled by the
// existing save/discard/cancel prompts; never write document contents here.
func saveReaderSession() error {
	windows := make([]savedReaderWindow, 0, len(readerWindows))
	for _, a := range readerWindows {
		if a.win.IsDestroyed() {
			continue
		}
		w := savedReaderWindow{Root: a.root, SidebarVisible: a.sidebarVisible, SidebarWidth: a.sidebarWidth}
		if a.doc != nil {
			w.Active = a.doc.path
		}
		for _, d := range a.tabs {
			w.Tabs = append(w.Tabs, savedReaderTab{Path: d.path, Scope: d.scopeRoot})
		}
		windows = append(windows, w)
	}
	data, err := json.Marshal(windows)
	if err != nil {
		return err
	}
	path, err := sessionPath()
	if err != nil {
		return err
	}
	return writePreferences(path, data)
}

func restoreReaderSession() bool {
	path, err := sessionPath()
	if err != nil {
		return false
	}
	data, err := readPreviewBytesNoHistory(path)
	if err != nil {
		return false
	}
	var windows []savedReaderWindow
	if json.Unmarshal(data, &windows) != nil {
		return false
	}
	for _, w := range windows {
		a := newReaderWindow(w.Root, false)
		// A file root opens its own tab; retain it only if it was in the session.
		for _, d := range append([]*document(nil), a.tabs...) {
			keep := false
			for _, tab := range w.Tabs {
				if tab.Path == d.path {
					keep = true
					break
				}
			}
			if !keep {
				a.removeTab(d)
			}
		}
		a.sidebarVisible = w.SidebarVisible
		if w.SidebarWidth >= 140 && w.SidebarWidth <= 600 {
			a.sidebarWidth = w.SidebarWidth
		}
		for _, tab := range w.Tabs {
			if !filepath.IsAbs(tab.Path) || !filepath.IsAbs(tab.Scope) {
				continue
			}
			if info, err := os.Stat(tab.Path); err == nil && !info.IsDir() {
				a.openDocumentAt(tab.Path, tab.Scope, "")
			}
		}
		for _, d := range a.tabs {
			if d.path == w.Active {
				a.activateDocument(d)
				break
			}
		}
		a.win.Invalidate()
	}
	if len(windows) == 0 {
		newReaderWindow("", false)
	}
	return true
}
