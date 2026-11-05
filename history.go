package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type openedItem struct {
	Path   string    `json:"path"`
	Folder bool      `json:"folder"`
	Opened time.Time `json:"opened"`
}

var openHistory []openedItem
var historyLoaded bool

func historyPath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "history.json")
}
func loadOpenHistory(legacy []string) {
	if historyLoaded {
		return
	}
	historyLoaded = true
	path := historyPath()
	if path == "" {
		return
	}
	data, err := readPreviewBytesNoHistory(path)
	if err == nil {
		_ = json.Unmarshal(data, &openHistory)
	} else if os.IsNotExist(err) {
		for _, p := range legacy {
			openHistory = append(openHistory, openedItem{Path: p, Folder: true})
		}
	}
	seen := map[string]bool{}
	clean := make([]openedItem, 0, 100)
	for _, item := range openHistory {
		if !filepath.IsAbs(item.Path) || seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		clean = append(clean, item)
		if len(clean) == 100 {
			break
		}
	}
	openHistory = clean
}
func readPreviewBytesNoHistory(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, fmt.Errorf("history file too large")
	}
	return os.ReadFile(path)
}
func recentHistoryFolders() []string {
	var out []string
	for _, item := range openHistory {
		if item.Folder {
			out = append(out, item.Path)
			if len(out) == maxRecentFolders {
				break
			}
		}
	}
	return out
}
func (a *application) saveHistory() {
	path := historyPath()
	if path == "" {
		return
	}
	data, err := json.Marshal(openHistory)
	if err == nil {
		err = writePreferences(path, data)
	}
	if err != nil {
		a.status = a.tr("Could not save history: ") + err.Error()
	}
	for _, other := range readerWindows {
		other.recentFolders = recentHistoryFolders()
		if other.historyWindow != nil {
			other.historyWindow.Invalidate()
		}
	}
	if activeReader != nil {
		activeReader.installMenu()
	}
}
func (a *application) rememberOpened(path string, folder bool) {
	items := []openedItem{{Path: path, Folder: folder, Opened: time.Now()}}
	for _, item := range openHistory {
		if item.Path != path {
			items = append(items, item)
		}
		if len(items) == 100 {
			break
		}
	}
	openHistory = items
	a.saveHistory()
}
func (a *application) clearHistory() {
	openHistory = nil
	a.saveHistory()
	a.storePreferences()
}
func (a *application) openHistoryItem(item openedItem, newWindow bool) {
	if a.fileOperation {
		return
	}
	info, err := os.Stat(item.Path)
	if err != nil || info.IsDir() != item.Folder {
		a.status = a.tr("This history item is no longer available.")
		a.win.Focus()
		a.win.Invalidate()
		return
	}
	if newWindow {
		newReaderWindow(item.Path, false)
	} else {
		a.setRoot(item.Path, item.Folder)
		a.win.Focus()
	}
}
func (a *application) recentMenuItems() []*mygo.MenuItem {
	if len(openHistory) == 0 {
		return []*mygo.MenuItem{{Label: a.tr("No recent items"), Disabled: true}}
	}
	var items []*mygo.MenuItem
	for i, item := range openHistory {
		if i == 10 {
			break
		}
		entry := item
		prefix := ""
		if entry.Folder {
			prefix = a.tr("Folder") + " · "
		}
		items = append(items, &mygo.MenuItem{Label: prefix + filepath.Base(entry.Path) + " · " + filepath.Base(filepath.Dir(entry.Path)), ToolTip: entry.Path, Click: func(*mygo.MenuItem, *mygo.Window) { a.openHistoryItem(entry, false) }})
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Label: a.tr("Open history…"), Click: func(*mygo.MenuItem, *mygo.Window) { a.showHistory() }}, &mygo.MenuItem{Label: a.tr("Clear history"), Click: func(*mygo.MenuItem, *mygo.Window) { a.clearHistory() }})
	return items
}
func (a *application) showHistory() {
	if a.historyWindow != nil && !a.historyWindow.IsDestroyed() {
		a.historyWindow.Focus()
		return
	}
	newWindow := false
	win := mygo.NewWindow(mygo.WindowOptions{Title: a.tr("Open history"), Width: 680, Height: 520, MinWidth: 500, MinHeight: 320, Content: ui.View(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Checkbox(c, &newWindow, a.tr("Open in a new window"))
				ui.Box(c).Grow(1)
				if ui.Button(c, a.tr("Clear history")).Disabled(len(openHistory) == 0).Clicked() {
					a.clearHistory()
				}
			})
			if len(openHistory) == 0 {
				ui.Text(c, a.tr("No recent items")).TextColor(c.Theme().TextMuted)
			}
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					for _, entry := range append([]openedItem(nil), openHistory...) {
						item := entry
						label := filepath.Base(item.Path)
						if item.Folder {
							label = a.tr("Folder") + " · " + label
						}
						button := ui.ButtonBase(c).FillWidth().Padding(10).Label(label)
						button.Children(func() {
							ui.Column(c).FillWidth().Gap(4).Children(func() {
								ui.Text(c, label).SingleLine()
								ui.Text(c, item.Path).FontSize(12).SingleLine().TextColor(c.Theme().TextMuted).Tooltip(item.Path)
								if !item.Opened.IsZero() {
									ui.Text(c, item.Opened.Local().Format("2006-01-02 15:04")).FontSize(11).TextColor(c.Theme().TextMuted)
								}
							})
						})
						if button.Clicked() {
							a.openHistoryItem(item, newWindow)
						}
					}
				})
			})
		})
	})})
	a.historyWindow = win
	win.OnFocus(func() { activeReader = a; a.installMenu() })
	win.OnClosed(func() {
		if a.historyWindow == win {
			a.historyWindow = nil
		}
	})
}
