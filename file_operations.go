package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *application) fileContextMenu(m *ui.Menu, path string) {
	isDir := a.knownFolders[path]
	if m.Item(a.tr("Open")).Chosen() {
		if isDir {
			a.fileOutline.Open.Add(path)
		} else {
			a.openDocument(path)
		}
	}
	if m.Item(a.tr("Open with default app")).Chosen() {
		go func() {
			if err := mygo.Shell.OpenPath(path); err != nil {
				a.win.Update(func() { a.fileError(err) })
			}
		}()
	}
	if m.Item(a.tr("Show in Finder")).Chosen() {
		mygo.Shell.ShowItemInFolder(path)
	}
	m.Separator()
	parent := filepath.Dir(path)
	if isDir {
		parent = path
	}
	a.newItemMenu(m, parent)
	m.Separator()
	protected := isDir && path == a.root
	if m.Item(a.tr("Rename…")).Disabled(protected || a.fileOperation).Chosen() {
		a.renameItem(path)
	}
	if m.Item(a.tr("Move to Trash…")).Disabled(protected || a.fileOperation).Chosen() {
		a.trashItem(path)
	}
	m.Separator()
	if m.Item(a.tr("Refresh folders")).Chosen() {
		a.refreshTree()
	}
}

func (a *application) folderContextMenu(m *ui.Menu, parent string) {
	a.newItemMenu(m, parent)
	m.Separator()
	if m.Item(a.tr("Refresh folders")).Chosen() {
		a.refreshTree()
	}
}

func (a *application) newItemMenu(m *ui.Menu, parent string) {
	if m.Item(a.tr("New file…")).Disabled(a.fileOperation).Chosen() {
		a.createItem(parent, false)
	}
	if m.Item(a.tr("New folder…")).Disabled(a.fileOperation).Chosen() {
		a.createItem(parent, true)
	}
}

func (a *application) fileError(err error) {
	message := a.tr(err.Error())
	if errors.Is(err, os.ErrExist) {
		message = a.tr("That name already exists. Choose another name.")
	}
	if errors.Is(err, os.ErrPermission) {
		message = a.tr("Permission denied. Check the folder’s permissions in Finder.")
	}
	if errors.Is(err, os.ErrNotExist) {
		message = a.tr("The item no longer exists. Refresh the folder.")
	}
	a.status = a.tr("Cannot change files: ") + message
	// Naming dialogs show errors inline. Operations without a naming dialog
	// still need visible feedback because the reader has no status bar.
	if !a.fileOperation && a.win != nil {
		go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageWarning,
			Message: a.tr("Cannot change files: "), Detail: message, Buttons: []string{a.tr("OK")}})
	}
}

func validItemName(name string) error {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." || strings.ContainsAny(name, "/:\x00\r\n") {
		return errors.New("Enter a name without slashes, colons, or line breaks.")
	}
	return nil
}

// Recheck the parent at execution time: an external tool may have replaced it
// with a link while the name dialog was open.
func checkedParent(root, parent string) error {
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	if resolved != parent || !pathInScope(root, parent) {
		return os.ErrPermission
	}
	info, err := os.Stat(parent)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("The parent is no longer a folder.")
	}
	return nil
}

func (a *application) nameItem(title, initial, parent string, submit func(string) error) {
	if a.fileOperation {
		return
	}
	a.fileOperation = true
	name, problem := initial, ""
	selectName := true
	var win *mygo.Window
	win = mygo.NewWindow(mygo.WindowOptions{
		Title: a.tr(title), Width: 440, Height: 210, MinWidth: 400, MinHeight: 210,
		Content: ui.View(func(c *ui.Context) {
			c.SetTheme(a.theme(c))
			ui.Column(c).Fill().Padding(20).Gap(12).Background(c.Theme().Background).Children(func() {
				ui.Text(c, parent).SingleLine().TextColor(c.Theme().TextMuted).Tooltip(parent)
				input := ui.TextInput(c, &name).FillWidth().Label(a.tr("Name")).AutoFocus()
				if selectName {
					input.SetTextSelection(0, len([]rune(name)))
					selectName = false
				}
				if problem != "" {
					ui.Text(c, problem).FontSize(12).TextColor(c.Theme().Danger)
				}
				ui.Spacer(c)
				ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c, a.tr("Cancel")).MinWidth(76).Height(30).Clicked() || c.Shortcut(0, ui.KeyEscape) {
						win.Close()
					}
					if ui.Button(c, a.tr("Confirm")).MinWidth(76).Height(30).Clicked() || input.Submitted() {
						if err := validItemName(name); err != nil {
							problem = a.tr(err.Error())
							return
						}
						if err := submit(name); err != nil {
							a.fileError(err)
							problem = a.status
							return
						}
						win.Close()
					}
				})
			})
		}),
	})
	a.filePrompt = win
	win.OnClosed(func() {
		a.filePrompt = nil
		a.fileOperation = false
		a.win.Invalidate()
	})
}

func (a *application) createItem(parent string, folder bool) {
	root := a.scopeRoot()
	title, name := "New file…", "Untitled.txt"
	if folder {
		title, name = "New folder…", "Untitled folder"
	}
	a.nameItem(title, a.tr(name), parent, func(name string) error {
		if a.scopeRoot() != root {
			return errors.New("The selected folder changed. Close this dialog and try again.")
		}
		if err := checkedParent(root, parent); err != nil {
			return err
		}
		path := filepath.Join(parent, name)
		if folder {
			if err := os.Mkdir(path, 0755); err != nil {
				return err
			}
		} else {
			// O_EXCL protects existing files, including dangling symlinks.
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		if a.rootIsFile {
			a.setRoot(parent, true)
		}
		a.fileOutline.Open.Add(parent)
		a.refreshTree()
		if folder {
			a.fileOutline.Open.Add(path)
		} else {
			a.openDocument(path)
		}
		return nil
	})
}

func withinItem(parent, path string) bool {
	return path == parent || strings.HasPrefix(path, parent+string(filepath.Separator))
}

func (a *application) itemHasEdits(path string) bool {
	for _, d := range a.tabs {
		if withinItem(path, d.path) && (documentModified(d) || d.saving) {
			return true
		}
	}
	return false
}

func (a *application) checkItem(root, path string, expected os.FileInfo) error {
	if err := checkedParent(root, filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, expected) {
		return errors.New("The item changed outside CatView. Refresh and try again.")
	}
	if a.itemHasEdits(path) {
		return errors.New("Save or discard edits in this item before renaming or moving it to Trash.")
	}
	return nil
}

func (a *application) renameItem(path string) {
	if a.fileOperation {
		return
	}
	if a.itemHasEdits(path) {
		a.fileError(errors.New("Save or discard edits in this item before renaming or moving it to Trash."))
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		a.fileError(err)
		return
	}
	root := a.scopeRoot()
	a.nameItem("Rename…", filepath.Base(path), filepath.Dir(path), func(name string) error {
		if a.scopeRoot() != root {
			return errors.New("The selected folder changed. Close this dialog and try again.")
		}
		if err := a.checkItem(root, path, info); err != nil {
			return err
		}
		destination := filepath.Join(filepath.Dir(path), name)
		if destination == path {
			return nil
		}
		if err := renameNoReplace(path, destination); err != nil {
			return err
		}
		a.loadEpoch++ // In-flight reads must not resurrect old paths.
		active := ""
		if a.doc != nil {
			active = a.doc.path
			if withinItem(path, active) {
				active = destination + strings.TrimPrefix(active, path)
			}
		}
		for folder := range a.knownFolders {
			if withinItem(path, folder) && a.fileOutline.Open.Has(folder) {
				a.fileOutline.Open.Remove(folder)
				a.fileOutline.Open.Add(destination + strings.TrimPrefix(folder, path))
			}
		}
		var reopen []string
		for _, d := range append([]*document(nil), a.tabs...) {
			if withinItem(path, d.path) {
				reopen = append(reopen, destination+strings.TrimPrefix(d.path, path))
				a.removeTab(d)
			}
		}
		if a.root == path {
			a.root = destination
		}
		a.refreshTree()
		for _, item := range reopen {
			if item != active {
				a.openDocument(item)
			}
		}
		if active != "" {
			a.openDocument(active)
		}
		return nil
	})
}

func (a *application) trashItem(path string) {
	if a.fileOperation {
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		a.fileError(err)
		return
	}
	root := a.scopeRoot()
	if err := a.checkItem(root, path, info); err != nil {
		a.fileError(err)
		return
	}
	a.fileOperation = true
	tr := a.translatorForUI()
	go func() {
		result, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: a.win, Type: mygo.MessageWarning,
			Message: fmt.Sprintf(tr("Move “%s” to Trash?"), filepath.Base(path)),
			Detail:  tr("The item and its contents can be restored from the Trash."),
			Buttons: []string{tr("Cancel"), tr("Move to Trash")}, DefaultButton: 0, CancelButton: 0,
		})
		a.win.Update(func() {
			if err != nil || result.Button != 1 {
				a.fileOperation = false
				if err != nil {
					a.fileError(err)
				}
				return
			}
			if err := a.checkItem(root, path, info); err != nil {
				a.fileOperation = false
				a.fileError(err)
				return
			}
			// Shell.TrashItem uses the system recoverable Trash, never unlink/remove.
			go func() {
				err := mygo.Shell.TrashItem(path)
				a.win.Update(func() {
					a.fileOperation = false
					if err != nil {
						a.fileError(err)
						return
					}
					a.loadEpoch++
					for _, d := range append([]*document(nil), a.tabs...) {
						if withinItem(path, d.path) {
							a.removeTab(d)
						}
					}
					if a.root == path {
						a.setRoot(filepath.Dir(path), true)
					}
					a.refreshTree()
				})
			}()
		})
	}()
}
