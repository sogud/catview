// CatView is a local-first file reader built with MyGo's native UI.
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	translatekit "github.com/sogud/translate-kit"
)

type application struct {
	translator        *translatekit.Client
	translationConfig translationSettings
	translationNotice string
	language          string
	resolvedLanguage  string
	win               *mygo.Window
	settings          *mygo.Window
	menu              *mygo.Menu
	shownStatus       string
	closePrompt       bool
	updates           updateController
	fileOperation     bool
	filePrompt        *mygo.Window
	settingsTab       int
	colorTheme        string
	preferencesError  string
	root              string
	rootIsFile        bool
	selected          string
	doc               *document
	tabs              []*document
	activeTab         int
	folders           map[string][]fileEntry
	folderErrors      map[string]string
	folderLoading     map[string]bool
	knownFolders      map[string]bool
	fileOutline       ui.OutlineState[string]
	fileRow           int
	loadSerial        uint64
	loadEpoch         uint64
	treeVersion       uint64
	sidebarVisible    bool
	sidebarWidth      float32
	themeMode         int // 0 system, 1 light, 2 dark
	accentIndex       int
	fileIconStyle     int // 0 system, 1 Material
	systemIcons       map[string]ui.ImageSource
	fontSize          float32
	lineHeight        float32
	readingWidth      int
	recentFolders     []string
	status            string
	pdf               *mygo.WebView
	htmlView          *mygo.WebView
	epubView          *mygo.WebView
	epubCancel        func()
	native            *nativePreview
}

func newApplication() *application {
	return &application{
		folders:        make(map[string][]fileEntry),
		folderErrors:   make(map[string]string),
		folderLoading:  make(map[string]bool),
		knownFolders:   make(map[string]bool),
		fileRow:        -1,
		sidebarVisible: true,
		sidebarWidth:   248,
		colorTheme:     colorThemes[0].id,
		accentIndex:    colorThemes[0].accent,
		fontSize:       15,
		lineHeight:     1.75,
		status:         "本地阅读 · 默认只读",
	}
}

// Accent choices tint selections without coloring the reading surface.
type accentPalette struct {
	name           string
	swatch         string
	lightSelection string
	darkSelection  string
	lightLink      string
	darkLink       string
}

var accentPalettes = [...]accentPalette{
	{"Graphite", "#73818A", "#E0E5E7", "#41494D", "#526C7B", "#B6CAD5"},
	{"Blue", "#6588A0", "#DCE7EE", "#3C505D", "#4A728B", "#A6C8DE"},
	{"Green", "#75957C", "#E0E9E1", "#405247", "#4F7861", "#ADD0B9"},
	{"Purple", "#9380AE", "#E8E1F0", "#4D425E", "#76538F", "#D0B5EC"},
	{"Amber", "#AE8A56", "#EEE5D7", "#564A36", "#80602E", "#DEC397"},
	{"Teal", "#648F91", "#DEEAEA", "#395354", "#3C7478", "#A0D2D1"},
	{"Rose", "#B17F89", "#F0E1E5", "#5B414A", "#965469", "#E7B1C1"},
	{"Orange", "#B3866B", "#F1E4DA", "#594739", "#905F3C", "#E2B995"},
}

func (a *application) palette() accentPalette {
	index := max(0, min(a.accentIndex, len(accentPalettes)-1))
	return accentPalettes[index]
}

func (a *application) linkColor(dark bool) ui.Color {
	p := a.palette()
	if dark {
		return ui.Hex(p.darkLink)
	}
	return ui.Hex(p.lightLink)
}

func (a *application) theme(c *ui.Context) *ui.Theme {
	t := *c.Theme()
	switch a.themeMode {
	case 1:
		t = *ui.LightTheme()
	case 2:
		t = *ui.DarkTheme()
	}
	p := a.palette()
	var selection ui.Color
	if t.Dark {
		selection = ui.Hex(p.darkSelection)
	} else {
		selection = ui.Hex(p.lightSelection)
	}
	colors := a.colors(t.Dark)
	t.Background = ui.Hex(colors.paper)
	t.Surface = ui.Hex(colors.surface)
	t.Text = ui.Hex(colors.ink)
	t.TextMuted = ui.Hex(colors.muted)
	t.Border = ui.Hex(colors.border)
	t.SurfaceHover = t.Surface.Mix(t.Text, 0.06)
	t.SurfacePressed = t.Surface.Mix(t.Text, 0.11)
	t.Accent = selection
	t.AccentHover = selection.Mix(t.Text, 0.09)
	t.AccentPressed = selection.Mix(t.Text, 0.17)
	t.AccentText = t.Text
	t.Selection = a.linkColor(t.Dark).Alpha(0.25)
	t.Focus = a.linkColor(t.Dark).Alpha(0.50)
	t.Spacing = 3.5
	return &t
}

var (
	sidebarGlyph = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/></svg>`))
	openGlyph    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v10H3z"/><path d="M3 19l3-8h15"/></svg>`))
)

func (a *application) view(c *ui.Context) {
	c.SetTheme(a.theme(c))
	t := c.Theme()
	root := ui.Column(c).Fill().AlignItems(ui.Stretch)
	root.Children(func() {
		if a.sidebarVisible {
			// Keep Split's drag target, keyboard resizing and focus ring,
			// without a permanent rule between the two surfaces.
			splitTheme := *t
			splitTheme.Border = ui.Transparent
			c.SetTheme(&splitTheme)
			pane := ui.Split(c, &a.sidebarWidth, func() {
				c.SetTheme(t)
				a.sidebar(c)
			}, func() {
				c.SetTheme(t)
				a.documentPane(c)
			}).Grow(1)
			c.SetTheme(t)
			if pane.Changed() {
				a.storePreferences()
			}
		} else {
			a.documentPane(c)
		}
	})
	// SetTheme also paints the root, so clear it after the scoped themes.
	if a.colorTheme == "macos" && c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	if paths := root.DroppedFiles(); len(paths) > 0 {
		if info, err := os.Stat(paths[0]); err == nil {
			a.setRoot(paths[0], info.IsDir())
		} else {
			a.status = "无法打开：" + err.Error()
		}
	}
	a.syncMenu()
	a.updateNotice(c)
	if a.shownStatus != a.status {
		a.shownStatus = a.status
		if strings.HasPrefix(a.status, "Cannot") || strings.HasPrefix(a.status, "Could not") || strings.HasPrefix(a.status, "無法") || strings.HasPrefix(a.status, "儲存失敗") || strings.HasPrefix(a.status, "无法") || strings.HasPrefix(a.status, "拒绝") ||
			strings.HasPrefix(a.status, "打开失败") || strings.HasPrefix(a.status, "保存失败") ||
			strings.Contains(a.status, "未保存") {
			c.Toast(a.status)
		}
	}
}

// Recent folders stay in the native File menu; the reading window still
// contains only one file tree and one content pane.
const maxRecentFolders = 8

func (a *application) rememberFolder(path string) {
	folders := []string{path}
	for _, previous := range a.recentFolders {
		if previous == path {
			continue
		}
		folders = append(folders, previous)
		if len(folders) == maxRecentFolders {
			break
		}
	}
	a.recentFolders = folders
	a.storePreferences()
}

func (a *application) recentMenuItems() []*mygo.MenuItem {
	if len(a.recentFolders) == 0 {
		return []*mygo.MenuItem{{Label: a.tr("No recent folders"), Disabled: true}}
	}
	items := make([]*mygo.MenuItem, 0, len(a.recentFolders)+2)
	for _, folder := range a.recentFolders {
		path := folder
		items = append(items, &mygo.MenuItem{
			Label:   filepath.Base(path) + " · " + filepath.Base(filepath.Dir(path)),
			ToolTip: path,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.setRoot(path, true)
				a.win.Invalidate()
			},
		})
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{
		Label: a.tr("Clear recent folders"),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.recentFolders = nil
			a.storePreferences()
			a.installMenu()
			a.win.Invalidate()
		},
	})
	return items
}

func (a *application) restoreSelectedFileRow() {
	a.fileRow = -1
	for i := 0; i < a.fileOutline.Rows(); i++ {
		if a.fileOutline.Item(i) == a.selected {
			a.fileRow = i
			return
		}
	}
}

func (a *application) toggleSidebar() {
	a.sidebarVisible = !a.sidebarVisible
	a.storePreferences()
	a.win.Invalidate()
}

func (a *application) toolbar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(float32(32), bar.Height)).
		Padding(0, 12, 0, bar.Left+8).AlignItems(ui.Center).
		DragWindow().Children(func() {
		ui.Toolbar(c, func() {
			if ui.Button(c, "").Size(28, 28).Padding(0).Label(a.tr("Show sidebar")).Tooltip(a.tr("Toggle sidebar ⌘B")).Children(func() {
				ui.Icon(c, sidebarGlyph).FontSize(18)
			}).Clicked() {
				a.toggleSidebar()
			}
			if ui.Button(c, "").Size(28, 28).Padding(0).Label(a.tr("Open folder…")).Tooltip(a.tr("Open folder ⇧⌘O")).Children(func() {
				ui.Icon(c, openGlyph).FontSize(18)
			}).Clicked() {
				a.choose(true)
			}
			ui.Spacer(c)
		}).FillWidth()
	})
}

// The native Outline keeps the familiar disclosure arrows and keyboard
// navigation while allowing icons before file names in the same sidebar.
func (a *application) sidebar(c *ui.Context) {
	t := c.Theme()
	sidebar := ui.Column(c).Fill().AlignItems(ui.Stretch)
	if a.colorTheme != "macos" || !c.Vibrancy() {
		sidebar.Background(t.Surface)
	}
	sidebar.Children(func() {
		a.toolbar(c)
		if a.root == "" {
			return
		}
		a.fileOutline.List.Selected = &a.fileRow
		a.fileOutline.List.Label = func(i int) string {
			if i < 0 || i >= a.fileOutline.Rows() {
				return ""
			}
			return filepath.Base(a.fileOutline.Item(i))
		}
		outline := ui.Outline(c, &a.fileOutline, []string{a.root}, a.outlineChildren, func(path string) {
			isFolder := a.knownFolders[path]
			icon := a.treeIcon(path, isFolder, a.fileOutline.Open.Has(path))
			row := ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(7).KeepFocus().Children(func() {
				if icon != nil {
					ui.Image(c, icon).Size(17, 17).Fit(ui.Contain).Shrink(0)
				}
				ui.Text(c, filepath.Base(path)).SingleLine().Grow(1).Tooltip(path)
			})
			row.ContextMenu(func(m *ui.Menu) { a.fileContextMenu(m, path) })
			// Only the content handles this click; the sibling disclosure arrow
			// keeps Outline's built-in behavior, including Option-click.
			if isFolder && row.Clicked() {
				for i := 0; i < a.fileOutline.Rows(); i++ {
					if a.fileOutline.Item(i) == path {
						a.fileRow = i
						break
					}
				}
				a.fileOutline.List.Focus(c)
				a.toggleFolder(path)
			}
		}).Grow(1).Padding(7, 6).ContextMenu(func(m *ui.Menu) { a.folderContextMenu(m, a.scopeRoot()) })

		if a.fileRow < 0 || a.fileRow >= a.fileOutline.Rows() {
			return
		}
		path := a.fileOutline.Item(a.fileRow)
		if outline.Changed() && !a.knownFolders[path] {
			a.openDocument(path)
		}
		if outline.Submitted() && a.knownFolders[path] {
			a.toggleFolder(path)
		}
	})
}

func (a *application) toggleFolder(path string) {
	if a.fileOutline.Open.Has(path) {
		a.fileOutline.Open.Remove(path)
	} else {
		a.fileOutline.Open.Add(path)
	}
}

// Return a non-nil slice for a directory, even when it is collapsed,
// so Outline shows the disclosure arrow without loading every subfolder.
func (a *application) outlineChildren(path string) []string {
	if !a.knownFolders[path] {
		return nil
	}
	rows, loaded := a.folders[path]
	if !loaded {
		if a.fileOutline.Open.Has(path) && !a.folderLoading[path] {
			if _, failed := a.folderErrors[path]; !failed {
				a.loadDirectory(path)
			}
		}
		return []string{}
	}
	children := make([]string, 0, len(rows))
	for _, entry := range rows {
		if entry.name == "" {
			continue
		}
		children = append(children, filepath.Join(path, entry.name))
	}
	return children
}

func (a *application) reader(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Key(a.selected).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		d := a.doc
		if d == nil {
			a.welcomeView(c)
			return
		}
		if a.previewMessage(c, d) {
			return
		}
		if d.systemPreview {
			a.nativePreviewView(c)
			return
		}
		if a.translationReader(c, d) {
			return
		}
		switch d.kind {
		case kindMarkdown:
			if d.editing {
				a.codeView(c, d)
			} else if d.htmlRich {
				if a.htmlView != nil {
					ui.WebView(c, a.htmlView).Grow(1)
				}
			} else {
				d.md.Theme = &markdown.Theme{
					FontSize: a.fontSize, LineHeight: a.lineHeight,
					Link: a.linkColor(t.Dark), BlockSpacing: 1.1,
					CodeSize: 1, CodeBlockBackground: t.Background,
				}
				ui.Scroll(c).Grow(1).Padding(30, 38).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(a.preferredTextWidth()).AlignSelf(ui.Center).Children(func() {
						markdown.View(c, d.source, &d.md)
					})
				})
			}
		case kindText:
			a.codeView(c, d)
		case kindHTML:
			if d.editing {
				a.codeView(c, d)
			} else if a.htmlView != nil {
				ui.WebView(c, a.htmlView).Grow(1)
			}
		case kindOffice:
			if a.htmlView != nil {
				ui.WebView(c, a.htmlView).Grow(1)
			}
		case kindDocument:
			if d.showSource && d.code != nil {
				a.codeView(c, d)
			} else if a.htmlView != nil {
				ui.WebView(c, a.htmlView).Grow(1)
			}
		case kindSVG:
			if a.htmlView != nil {
				ui.WebView(c, a.htmlView).Grow(1)
			}
		case kindTable, kindArchive:
			a.tableView(c, d)
		case kindMedia, kindSystem:
			a.nativePreviewView(c)
		case kindImage:
			if d.imageAnimated && a.htmlView != nil {
				ui.WebView(c, a.htmlView).Grow(1)
			} else {
				a.imageView(c, d)
			}
		case kindPDF:
			if a.pdf != nil {
				ui.WebView(c, a.pdf).Grow(1)
			} else {
				a.infoView(c, a.tr("Opening PDF…"), a.tr("Using the system PDF viewer."))
			}
		case kindEPUB:
			if a.epubView != nil {
				ui.WebView(c, a.epubView).Grow(1)
			} else {
				a.infoView(c, a.tr("EPUB preview unavailable"), a.tr("Try another EPUB file."))
			}
		default:
			a.infoView(c, a.tr("Preview unavailable"), a.tr("Open with another app in Finder."))
			ui.Row(c).Center().Children(func() {
				if ui.Button(c, a.tr("Show in Finder")).Clicked() {
					mygo.Shell.ShowItemInFolder(d.path)
				}
			})
		}
	})
}

func (a *application) codeView(c *ui.Context, d *document) {
	if d.code == nil {
		return
	}
	state := d.visibleCode()
	state.ReadOnly = !d.editing || a.fileOperation
	state.Font.Size = a.fontSize
	state.LineHeight = a.lineHeight
	editor.View(c, state).Grow(1).FillWidth()
}

func (a *application) infoView(c *ui.Context, heading, detail string) {
	ui.Column(c).Padding(24).Gap(12).Center().Children(func() {
		ui.Text(c, heading).FontSize(19).Bold()
		ui.Text(c, detail).TextColor(c.Theme().TextMuted)
	})
}

func (a *application) choose(directory bool) {
	if a.fileOperation {
		return
	}
	win := a.win
	go func() {
		title := a.tr("Choose a file")
		if directory {
			title = a.tr("Choose a folder")
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: title, Directory: directory})
		if err != nil {
			win.Update(func() { a.status = "无法打开选择器：" + err.Error() })
			return
		}
		if len(paths) == 0 {
			return
		}
		win.Update(func() { a.setRoot(paths[0], directory) })
	}()
}

func (a *application) toggleEditing() {
	if a.fileOperation {
		return
	}
	d := a.doc
	if d == nil || !d.canEdit() {
		return
	}
	if d.editing {
		a.cancelEditing()
	} else {
		clearTranslation(d)
		d.editing = true
	}
	a.win.Invalidate()
}

func (a *application) refreshTree() {
	a.fileRow = -1
	a.treeVersion++
	a.folders = make(map[string][]fileEntry)
	a.folderErrors = make(map[string]string)
	a.folderLoading = make(map[string]bool)
	a.knownFolders = make(map[string]bool)
	a.systemIcons = nil
	if a.root != "" && !a.rootIsFile {
		a.knownFolders[a.root] = true
	}
	a.win.Invalidate()
}

func (a *application) adjustFont(delta float32) {
	a.fontSize = max(float32(12), min(float32(24), a.fontSize+delta))
	a.epubSetFont()
	a.storePreferences()
	a.win.Invalidate()
}

func (a *application) setAppearance(mode int) {
	a.themeMode = mode
	a.applySystemAppearance()
	a.epubSetTheme()
	a.storePreferences()
	a.win.Invalidate()
}

func (a *application) setAccent(index int) {
	a.accentIndex = index
	a.storePreferences()
	a.win.Invalidate()
}

func (a *application) syncMenu() {
	if a.menu == nil || a.win == nil {
		return
	}
	a.syncTranslationMenu()
	d := a.doc
	canEdit := d != nil && d.canEdit()
	editing := canEdit && d.editing
	dirty := canEdit && d.code != nil && d.code.Buffer.Modified()
	for _, state := range []struct {
		id      string
		enabled bool
	}{
		{"edit", canEdit},
		{"save", editing && dirty && !a.fileOperation},
		{"reveal", d != nil},
		{"refresh", a.root != "" && !a.rootIsFile},
		{"find", d != nil && !d.systemPreview && d.code != nil && (d.kind == kindText || d.editing || d.showSource)},
		{"image-fit", d != nil && (d.kind == kindImage || d.kind == kindSVG)},
		{"image-size", d != nil && (d.kind == kindImage || d.kind == kindSVG)},
		{"preview-source", d != nil && !d.systemPreview && !d.loading && (d.kind == kindTable || d.kind == kindDocument || d.formatted != nil) && !d.editing && d.code != nil},
		{"system-preview", canUseSystemPreview(d)},
	} {
		item := a.menu.ItemByID(state.id)
		if item != nil && item.IsEnabled() != state.enabled {
			item.SetEnabled(state.enabled)
		}
	}
	edit := a.menu.ItemByID("edit")
	if item := a.menu.ItemByID("preview-source"); item != nil {
		item.SetChecked(d != nil && d.showSource)
	}
	if item := a.menu.ItemByID("system-preview"); item != nil {
		item.SetChecked(d != nil && d.systemPreview)
	}
	if edit != nil {
		label := a.tr("Edit file")
		if editing {
			label = a.tr("Finish editing")
		}
		if edit.Label != label {
			edit.SetLabel(label)
		}
	}
	if a.menu.ItemByID("sidebar").IsChecked() != a.sidebarVisible {
		a.menu.ItemByID("sidebar").SetChecked(a.sidebarVisible)
	}
	if a.win.IsDocumentEdited() != dirty {
		a.win.SetDocumentEdited(dirty)
	}
}

func (a *application) installMenu() {
	action := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			fn()
			a.win.Invalidate()
		}
	}
	readingItems := a.epubReadMenuItems()
	readingHidden := a.doc == nil || a.doc.kind != kindEPUB
	if a.doc != nil && a.doc.imagePages > 1 {
		readingItems = a.imageReadMenuItems()
		readingHidden = false
	}
	a.menu = mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(),
			{Label: a.tr("Settings"), Accelerator: "CmdOrCtrl+,", Click: action(a.showSettings)},
			{ID: "updates", Label: a.tr("Check for updates…"), Click: action(a.checkForUpdates)},
			mygo.Separator(), {Role: mygo.RoleServices},
			mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Label: a.tr("File"), Submenu: []*mygo.MenuItem{
			{Label: a.tr("Open file…"), Accelerator: "CmdOrCtrl+O", Click: action(func() { a.choose(false) })},
			{Label: a.tr("Open folder…"), Accelerator: "CmdOrCtrl+Shift+O", Click: action(func() { a.choose(true) })},
			{Label: a.tr("Recent folders"), Submenu: a.recentMenuItems()},
			mygo.Separator(),
			{ID: "edit", Label: a.tr("Edit file"), Disabled: true, Click: action(a.toggleEditing)},
			{ID: "save", Label: a.tr("Save"), Accelerator: "CmdOrCtrl+S", Disabled: true, Click: action(a.saveDocument)},
			{ID: "reveal", Label: a.tr("Show in Finder"), Disabled: true, Click: action(func() {
				if a.doc != nil {
					mygo.Shell.ShowItemInFolder(a.doc.path)
				}
			})},
			mygo.Separator(),
			{Label: a.tr("Close tab"), Accelerator: "CmdOrCtrl+W", Click: func(_ *mygo.MenuItem, window *mygo.Window) {
				if window != nil && window != a.win {
					window.Close()
				} else if a.doc != nil {
					a.requestCloseTab(a.doc, false)
				} else {
					a.win.Close()
				}
			}},
			{Label: a.tr("Close window"), Accelerator: "CmdOrCtrl+Shift+W", Click: action(func() { a.win.Close() })},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: a.tr("View"), Submenu: []*mygo.MenuItem{
			{ID: "sidebar", Label: a.tr("Show sidebar"), Type: mygo.MenuItemCheckbox,
				Checked: true, Accelerator: "CmdOrCtrl+B", Click: action(a.toggleSidebar)},
			{ID: "refresh", Label: a.tr("Refresh folders"), Accelerator: "CmdOrCtrl+R", Disabled: true, Click: action(a.refreshTree)},
			{ID: "find", Label: a.tr("Find"), Accelerator: "CmdOrCtrl+F", Disabled: true, Click: action(func() {
				if a.doc != nil && a.doc.code != nil {
					a.doc.visibleCode().Find("")
				}
			})},
			mygo.Separator(),
			{Label: a.tr("Zoom in / increase text size"), Accelerator: "CmdOrCtrl+Plus", Click: action(func() { a.adjustContentSize(1) })},
			{Label: a.tr("Zoom out / decrease text size"), Accelerator: "CmdOrCtrl+-", Click: action(func() { a.adjustContentSize(-1) })},
			{ID: "image-fit", Label: a.tr("Fit image to window"), Accelerator: "CmdOrCtrl+0", Click: action(func() { a.zoomImage(0) })},
			{ID: "image-size", Label: a.tr("Image at 100%"), Accelerator: "CmdOrCtrl+1", Click: action(func() { a.zoomImage(1) })},
			{ID: "preview-source", Label: a.tr("Show source"), Type: mygo.MenuItemCheckbox, Click: action(a.togglePreviewSource)},
			{ID: "system-preview", Label: a.tr("System preview"), Type: mygo.MenuItemCheckbox, Click: action(a.toggleSystemPreview)},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: a.tr("Reading"), Hidden: readingHidden, Submenu: readingItems},
		{Label: a.tr("Translation"), Submenu: a.translationMenuItems()},
		{Role: mygo.RoleWindowMenu},
	})
	mygo.App.SetMenu(a.menu)
}

type savedPreferences struct {
	ColorTheme     string   `json:"color_theme,omitempty"`
	Language       string   `json:"language,omitempty"`
	SidebarVisible bool     `json:"sidebar_visible"`
	SidebarWidth   float32  `json:"sidebar_width"`
	ThemeMode      int      `json:"theme_mode"`
	AccentIndex    int      `json:"accent_index"`
	FileIconStyle  int      `json:"file_icon_style"`
	FontSize       float32  `json:"font_size"`
	LineHeight     float32  `json:"line_height"`
	ReadingWidth   int      `json:"reading_width"`
	RecentFolders  []string `json:"recent_folders,omitempty"`
}

func (a *application) preferencesPath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "appearance.json")
}

func (a *application) loadPreferences() {
	path := a.preferencesPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var p savedPreferences
	if json.Unmarshal(data, &p) != nil {
		return
	}
	a.language = p.Language
	a.colorTheme = p.ColorTheme
	a.colorTheme = a.selectedTheme().id
	a.sidebarVisible = p.SidebarVisible
	if p.SidebarWidth >= 160 && p.SidebarWidth <= 440 {
		a.sidebarWidth = p.SidebarWidth
	}
	if p.ThemeMode >= 0 && p.ThemeMode < 3 {
		a.themeMode = p.ThemeMode
	}
	if p.FileIconStyle == 0 || p.FileIconStyle == 1 {
		a.fileIconStyle = p.FileIconStyle
	}
	if p.AccentIndex >= 0 && p.AccentIndex < len(accentPalettes) {
		a.accentIndex = p.AccentIndex
	}
	if p.FontSize >= 12 && p.FontSize <= 24 {
		a.fontSize = p.FontSize
	}
	if p.LineHeight >= 1.4 && p.LineHeight <= 2 {
		a.lineHeight = p.LineHeight
	}
	if p.ReadingWidth == 0 || p.ReadingWidth == 1 {
		a.readingWidth = p.ReadingWidth
	}
	seen := make(map[string]bool)
	for _, folder := range p.RecentFolders {
		if seen[folder] {
			continue
		}
		info, err := os.Stat(folder)
		if err != nil || !info.IsDir() {
			continue
		}
		a.recentFolders = append(a.recentFolders, folder)
		seen[folder] = true
		if len(a.recentFolders) == maxRecentFolders {
			break
		}
	}
}

func (a *application) storePreferences() {
	path := a.preferencesPath()
	if path == "" {
		return
	}
	p := savedPreferences{Language: a.language, ColorTheme: a.colorTheme,
		SidebarVisible: a.sidebarVisible, SidebarWidth: a.sidebarWidth,
		ThemeMode: a.themeMode, AccentIndex: a.accentIndex, FontSize: a.fontSize,
		LineHeight: a.lineHeight, ReadingWidth: a.readingWidth,
		RecentFolders: a.recentFolders, FileIconStyle: a.fileIconStyle,
	}
	a.preferencesError = ""
	data, err := json.Marshal(p)
	if err == nil {
		err = writePreferences(path, data)
	}
	if err != nil {
		a.preferencesError = a.tr("Could not save settings") + ": " + err.Error()
	}
}

func writePreferences(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".appearance-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (a *application) guardClose() {
	a.win.OnClose(func(ev *mygo.CloseEvent) {
		if a.fileOperation {
			ev.PreventDefault()
			if a.filePrompt != nil {
				a.filePrompt.Focus()
			}
			return
		}
		for _, d := range a.tabs {
			if documentModified(d) || d.saving {
				ev.PreventDefault()
				a.requestCloseTab(d, true)
				return
			}
		}
	})
}

func main() {
	app := newApplication()
	var pendingPaths []string
	mygo.App.OnOpenFile(func(path string) {
		if app.win == nil {
			pendingPaths = append(pendingPaths, path)
			return
		}
		if info, err := os.Stat(path); err == nil {
			app.setRoot(path, info.IsDir())
			app.win.Focus()
		}
	})
	mygo.App.SetVersion(releaseVersion)
	mygo.App.WhenReady(func() {
		app.loadPreferences()
		app.applySystemAppearance()
		app.resolveLanguage()
		app.loadTranslationSettings()
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "CatView", Width: 1120, Height: 750, MinWidth: 680, MinHeight: 440,
			TitleBarStyle: mygo.TitleBarHidden,
			Vibrancy:      app.sidebarMaterial(),
			StateKey:      "main", Content: ui.View(app.view),
		})
		app.installMenu()
		app.guardClose()
		app.startUpdates()
		app.win.OnClosed(func() {
			app.stopUpdates()
			for _, d := range app.tabs {
				if d.loadCancel != nil {
					d.loadCancel()
				}
				cancelTranslation(d)
			}
			app.closeNativePreview()
			app.closeEPUB()
			app.closeHTMLPreview()
			if app.pdf != nil {
				app.pdf.Destroy()
				app.pdf = nil
			}
			if app.settings != nil && !app.settings.IsDestroyed() {
				app.settings.Close()
			}
		})
		initialPath := ""
		if len(pendingPaths) > 0 {
			initialPath = pendingPaths[0]
		} else if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
			initialPath = os.Args[1]
		} else if len(app.recentFolders) > 0 {
			initialPath = app.recentFolders[0]
		}
		if initialPath != "" {
			path, err := filepath.Abs(initialPath)
			if err == nil {
				info, err := os.Stat(path)
				if err == nil {
					app.setRoot(path, info.IsDir())
					if info.IsDir() {
						// setRoot resolves symlinks, so preload using its canonical path.
						rootPath := app.root
						if rows, err := directoryEntries(rootPath); err == nil {
							app.folders[rootPath] = rows
							for _, entry := range rows {
								if entry.dir {
									app.knownFolders[filepath.Join(rootPath, entry.name)] = true
								}
							}
						}
					}
				}
			}
		}
		for i, path := range pendingPaths {
			if i == 0 {
				continue // The first file was opened above.
			}
			if info, err := os.Stat(path); err == nil {
				app.setRoot(path, info.IsDir())
			}
		}
		pendingPaths = nil
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
