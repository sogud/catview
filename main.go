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
)

type application struct {
	win            *mygo.Window
	settings       *mygo.Window
	menu           *mygo.Menu
	shownStatus    string
	closePrompt    bool
	root           string
	rootIsFile     bool
	selected       string
	doc            *document
	folders        map[string][]fileEntry
	folderErrors   map[string]string
	folderLoading  map[string]bool
	knownFolders   map[string]bool
	fileOutline    ui.OutlineState[string]
	fileRow        int
	loadSerial     uint64
	treeVersion    uint64
	sidebarVisible bool
	sidebarWidth   float32
	themeMode      int // 0 system, 1 light, 2 dark
	accentIndex    int
	fontSize       float32
	lineHeight     float32
	readingWidth   int
	recentFolders  []string
	status         string
	pdf            *mygo.WebView
	htmlView       *mygo.WebView
	epubView       *mygo.WebView
	epubCancel     func()
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
	{"石墨", "#73818A", "#E0E5E7", "#41494D", "#526C7B", "#B6CAD5"},
	{"雾蓝", "#6588A0", "#DCE7EE", "#3C505D", "#4A728B", "#A6C8DE"},
	{"苔绿", "#75957C", "#E0E9E1", "#405247", "#4F7861", "#ADD0B9"},
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
		// The reading surface remains neutral; only selected controls
		// take the muted chosen accent.
		t.Background = ui.Hex("#202225")
		t.Surface = ui.Hex("#2A2C2F")
		t.SurfaceHover = ui.Hex("#36393C")
		t.SurfacePressed = ui.Hex("#414549")
		t.Border = ui.Hex("#3C4144")
		t.Text = ui.Hex("#EDF0F1")
		t.TextMuted = ui.Hex("#ABB3B9")
		selection = ui.Hex(p.darkSelection)
	} else {
		t.Background = ui.Hex("#FBFAF7")
		t.Surface = ui.Hex("#F1F2F1")
		t.SurfaceHover = ui.Hex("#E7EAE8")
		t.SurfacePressed = ui.Hex("#DBE0DD")
		t.Border = ui.Hex("#E0E4E1")
		t.Text = ui.Hex("#2B3034")
		t.TextMuted = ui.Hex("#69747B")
		selection = ui.Hex(p.lightSelection)
	}
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
	root := ui.Column(c).Fill().AlignItems(ui.Stretch).Background(t.Background)
	root.Children(func() {
		a.toolbar(c)
		if a.sidebarVisible {
			pane := ui.Split(c, &a.sidebarWidth, func() { a.sidebar(c) }, func() { a.reader(c) }).Grow(1)
			if pane.Changed() {
				a.storePreferences()
			}
		} else {
			a.reader(c)
		}
	})
	if paths := root.DroppedFiles(); len(paths) > 0 {
		if info, err := os.Stat(paths[0]); err == nil {
			a.setRoot(paths[0], info.IsDir())
		} else {
			a.status = "无法打开：" + err.Error()
		}
	}
	a.syncMenu()
	if a.shownStatus != a.status {
		a.shownStatus = a.status
		if strings.HasPrefix(a.status, "无法") || strings.HasPrefix(a.status, "拒绝") ||
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
		return []*mygo.MenuItem{{Label: "无最近文件夹", Disabled: true}}
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
		Label: "清除最近记录",
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
	t := c.Theme()
	bar := c.TitleBar()
	ui.Row(c).Height(max(float32(32), bar.Height)).
		Padding(0, 12, 0, bar.Left+8).AlignItems(ui.Center).
		Background(t.Surface).
		DragWindow().Children(func() {
		ui.Toolbar(c, func() {
			if ui.Button(c, "").Label("显示侧边栏").Tooltip("显示或隐藏侧边栏 ⌘B").Children(func() {
				ui.Icon(c, sidebarGlyph).FontSize(18)
			}).Clicked() {
				a.toggleSidebar()
			}
			if ui.Button(c, "").Label("打开文件夹").Tooltip("打开文件夹 ⇧⌘O").Children(func() {
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
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(t.Surface).Children(func() {
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
			icon := iconForFile(path, isFolder, a.fileOutline.Open.Has(path))
			ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
				if icon != nil {
					ui.Image(c, icon).Size(17, 17).Fit(ui.Contain).Shrink(0)
				}
				ui.Text(c, filepath.Base(path)).SingleLine().Grow(1).Tooltip(path)
			})
		}).Grow(1).Padding(7, 6)

		if a.fileRow < 0 || a.fileRow >= a.fileOutline.Rows() {
			return
		}
		path := a.fileOutline.Item(a.fileRow)
		if outline.Changed() && !a.knownFolders[path] {
			a.openDocument(path)
		}
		if outline.Submitted() && a.knownFolders[path] {
			if a.fileOutline.Open.Has(path) {
				a.fileOutline.Open.Remove(path)
			} else {
				a.fileOutline.Open.Add(path)
			}
		}
	})
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
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		d := a.doc
		if d == nil {
			prompt := "⌘O 打开文件"
			if a.root != "" {
				prompt = "选择文件"
			}
			ui.Column(c).Fill().Center().Children(func() {
				ui.Text(c, prompt).TextColor(t.TextMuted)
			})
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
				}
				ui.Scroll(c).Grow(1).Padding(30, 38).Children(func() {
					ui.Column(c).MaxWidth(a.preferredTextWidth()).AlignSelf(ui.Center).Children(func() {
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
		case kindImage:
			if d.picture != nil {
				ui.Image(c, d.picture).Fill().Fit(ui.Contain).Padding(22)
			}
		case kindPDF:
			if a.pdf != nil {
				ui.WebView(c, a.pdf).Grow(1)
			} else {
				a.infoView(c, "正在打开 PDF…", "将使用系统内置的 PDF 阅读能力。")
			}
		case kindEPUB:
			if a.epubView != nil {
				ui.WebView(c, a.epubView).Grow(1)
			} else {
				a.infoView(c, "EPUB 无法预览", "请尝试其他 EPUB 文件。")
			}
		default:
			a.infoView(c, "暂不支持预览", "可以通过访达使用其他应用打开。")
			ui.Row(c).Center().Children(func() {
				if ui.Button(c, "在访达中显示").Clicked() {
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
	d.code.ReadOnly = !d.editing
	d.code.Font.Size = a.fontSize
	d.code.LineHeight = a.lineHeight
	editor.View(c, d.code).Grow(1).FillWidth()
}

func (a *application) infoView(c *ui.Context, heading, detail string) {
	ui.Column(c).Padding(24).Gap(12).Center().Children(func() {
		ui.Text(c, heading).FontSize(19).Bold()
		ui.Text(c, detail).TextColor(c.Theme().TextMuted)
	})
}

func (a *application) choose(directory bool) {
	win := a.win
	go func() {
		title := "选择文件"
		if directory {
			title = "选择文件夹"
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
	d := a.doc
	if d == nil || !d.canEdit() {
		return
	}
	if d.editing {
		a.cancelEditing()
	} else {
		d.editing = true
	}
	a.win.Invalidate()
}

func (a *application) refreshTree() {
	a.treeVersion++
	a.folders = make(map[string][]fileEntry)
	a.folderErrors = make(map[string]string)
	a.folderLoading = make(map[string]bool)
	a.knownFolders = make(map[string]bool)
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
	d := a.doc
	canEdit := d != nil && d.canEdit()
	editing := canEdit && d.editing
	dirty := canEdit && d.code != nil && d.code.Buffer.Modified()
	for _, state := range []struct {
		id      string
		enabled bool
	}{
		{"edit", canEdit},
		{"save", editing && dirty},
		{"reveal", d != nil},
		{"refresh", a.root != "" && !a.rootIsFile},
		{"find", d != nil && d.code != nil && (d.kind == kindText || d.editing)},
	} {
		item := a.menu.ItemByID(state.id)
		if item != nil && item.IsEnabled() != state.enabled {
			item.SetEnabled(state.enabled)
		}
	}
	edit := a.menu.ItemByID("edit")
	if edit != nil {
		label := "编辑文件"
		if editing {
			label = "结束编辑"
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
	a.menu = mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(),
			{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: action(a.showSettings)},
			mygo.Separator(), {Role: mygo.RoleServices},
			mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "打开文件…", Accelerator: "CmdOrCtrl+O", Click: action(func() { a.choose(false) })},
			{Label: "打开文件夹…", Accelerator: "CmdOrCtrl+Shift+O", Click: action(func() { a.choose(true) })},
			{Label: "最近打开的文件夹", Submenu: a.recentMenuItems()},
			mygo.Separator(),
			{ID: "edit", Label: "编辑文件", Disabled: true, Click: action(a.toggleEditing)},
			{ID: "save", Label: "保存", Accelerator: "CmdOrCtrl+S", Disabled: true, Click: action(a.saveDocument)},
			{ID: "reveal", Label: "在访达中显示", Disabled: true, Click: action(func() {
				if a.doc != nil {
					mygo.Shell.ShowItemInFolder(a.doc.path)
				}
			})},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "显示", Submenu: []*mygo.MenuItem{
			{ID: "sidebar", Label: "显示侧边栏", Type: mygo.MenuItemCheckbox,
				Checked: true, Accelerator: "CmdOrCtrl+B", Click: action(a.toggleSidebar)},
			{ID: "refresh", Label: "重新读取目录", Accelerator: "CmdOrCtrl+R", Disabled: true, Click: action(a.refreshTree)},
			{ID: "find", Label: "查找", Accelerator: "CmdOrCtrl+F", Disabled: true, Click: action(func() {
				if a.doc != nil && a.doc.code != nil {
					a.doc.code.Find("")
				}
			})},
			mygo.Separator(),
			{Label: "放大文字", Accelerator: "CmdOrCtrl+Plus", Click: action(func() { a.adjustFont(1) })},
			{Label: "缩小文字", Accelerator: "CmdOrCtrl+-", Click: action(func() { a.adjustFont(-1) })},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "阅读", Hidden: a.doc == nil || a.doc.kind != kindEPUB,
			Submenu: a.epubReadMenuItems()},
		{Role: mygo.RoleWindowMenu},
	})
	mygo.App.SetMenu(a.menu)
}

type savedPreferences struct {
	SidebarVisible bool     `json:"sidebar_visible"`
	SidebarWidth   float32  `json:"sidebar_width"`
	ThemeMode      int      `json:"theme_mode"`
	AccentIndex    int      `json:"accent_index"`
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
	a.sidebarVisible = p.SidebarVisible
	if p.SidebarWidth >= 160 && p.SidebarWidth <= 440 {
		a.sidebarWidth = p.SidebarWidth
	}
	if p.ThemeMode >= 0 && p.ThemeMode < 3 {
		a.themeMode = p.ThemeMode
	}
	if p.AccentIndex >= 0 && p.AccentIndex < 3 {
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
	p := savedPreferences{
		SidebarVisible: a.sidebarVisible, SidebarWidth: a.sidebarWidth,
		ThemeMode: a.themeMode, AccentIndex: a.accentIndex, FontSize: a.fontSize,
		LineHeight: a.lineHeight, ReadingWidth: a.readingWidth,
		RecentFolders: a.recentFolders,
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func (a *application) guardClose() {
	a.win.OnClose(func(ev *mygo.CloseEvent) {
		d := a.doc
		if d == nil || d.code == nil || !d.code.Buffer.Modified() {
			return
		}
		ev.PreventDefault()
		if a.closePrompt {
			return
		}
		a.closePrompt = true
		win := a.win
		content := d.code.Buffer.String()
		go func() {
			res, err := mygo.Dialog.Message(mygo.MessageOptions{
				Parent: win, Type: mygo.MessageWarning,
				Message:       "保存对文件的修改？",
				Buttons:       []string{"保存", "不保存", "取消"},
				DefaultButton: 0, CancelButton: 2,
			})
			if err == nil {
				switch res.Button {
				case 0:
					_, err = safeSaveText(d.path, d.digest, d.mode, content)
					if err == nil {
						win.Destroy()
						return
					}
					mygo.Dialog.Error("保存失败", err.Error())
				case 1:
					win.Destroy()
					return
				}
			}
			win.Update(func() { a.closePrompt = false })
		}()
	})
}

func main() {
	app := newApplication()
	mygo.App.WhenReady(func() {
		app.loadPreferences()
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "CatView", Width: 1120, Height: 750, MinWidth: 680, MinHeight: 440,
			TitleBarStyle: mygo.TitleBarHidden,
			StateKey:      "main", Content: ui.View(app.view),
		})
		app.installMenu()
		app.guardClose()
		app.win.OnClosed(func() {
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
		if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
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
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
