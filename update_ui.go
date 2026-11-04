package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *application) updateStatus() string {
	switch a.updates.phase {
	case "checking":
		return a.tr("Checking for updates…")
	case "downloading":
		return a.tr("Downloading update…")
	case "installing":
		return a.tr("Installing update…")
	case "ready":
		return a.tr("Update ready to install")
	case "available":
		return a.tr("An update is available")
	case "current":
		return a.tr("You’re up to date")
	case "error":
		return a.tr("Could not check for updates")
	}
	return a.tr("Software update")
}

func (a *application) showUpdateWindow() {
	u := &a.updates
	if u.window != nil && !u.window.IsDestroyed() {
		u.window.Show()
		u.window.Focus()
		return
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: a.tr("Software update"), Width: 460, Height: 300,
		MinWidth: 420, MinHeight: 280, DisableMaximize: true,
		Content: ui.View(a.updateView),
	})
	u.window = win
	win.OnClose(func(e *mygo.CloseEvent) {
		if u.phase == "installing" {
			e.PreventDefault()
		}
	})
	win.OnClosed(func() {
		if u.window == win {
			u.window = nil
		}
	})
}

func (a *application) updateView(c *ui.Context) {
	u, t := &a.updates, c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(14).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, a.updateStatus()).FontSize(17).Bold()
		version := "CatView " + releaseVersion
		if u.update != nil {
			version += " → " + u.update.Version
		}
		ui.Text(c, version).TextColor(t.TextMuted)
		if u.busy {
			progress := float64(-1)
			if u.phase == "downloading" && u.total > 0 {
				progress = float64(u.downloaded) / float64(u.total)
			}
			ui.Progress(c, progress).FillWidth()
			if u.phase == "downloading" {
				ui.Text(c, fmt.Sprintf("%.1f / %.1f MB", float64(u.downloaded)/1e6, float64(u.total)/1e6)).FontSize(12).TextColor(t.TextMuted)
			}
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				if u.phase == "ready" {
					ui.Text(c, a.tr("The update is downloaded and verified. Install when you are ready."))
				}
				if u.detail != "" {
					ui.Text(c, u.detail).TextColor(t.Danger)
				}
				if u.update != nil && u.update.Notes != "" {
					ui.Text(c, u.update.Notes).FontSize(12).TextColor(t.TextMuted)
				}
			})
		})
		ui.Row(c).FillWidth().Gap(10).Justify(ui.End).Children(func() {
			switch u.phase {
			case "ready":
				if ui.Button(c, a.tr("Later")).MinWidth(76).Clicked() {
					u.window.Close()
				}
				if ui.Button(c, a.tr("Install and Restart")).MinWidth(120).Clicked() {
					a.installUpdate()
				}
			case "available":
				if ui.Button(c, a.tr("Later")).MinWidth(76).Clicked() {
					u.window.Close()
				}
				if ui.Button(c, a.tr("Download update")).MinWidth(110).Clicked() {
					a.downloadUpdate()
				}
			case "checking", "downloading":
				if ui.Button(c, a.tr("Cancel")).MinWidth(76).Clicked() && u.cancel != nil {
					u.cancel()
				}
			case "installing":
				ui.Button(c, a.tr("Install and Restart")).Disabled(true)
			case "error":
				if ui.Button(c, a.tr("Open download page")).Clicked() {
					_ = mygo.Shell.OpenExternal(releasePage)
				}
				if ui.Button(c, a.tr("Retry")).MinWidth(76).Clicked() {
					a.beginUpdateCheck()
				}
			default:
				if ui.Button(c, a.tr("OK")).MinWidth(76).Clicked() {
					u.window.Close()
				}
			}
		})
	})
}

func (a *application) updateSettings(c *ui.Context) {
	u := &a.updates
	ui.Column(c).Gap(16).Children(func() {
		ui.Text(c, "CatView "+releaseVersion).Bold()
		if ui.Checkbox(c, &u.prefs.AutomaticChecks, a.tr("Automatically check for updates")).Changed() {
			a.storeUpdatePreferences()
			if u.prefs.AutomaticChecks {
				a.beginUpdateCheck()
			}
		}
		if ui.Checkbox(c, &u.prefs.AutomaticDownloads, a.tr("Automatically download updates")).Changed() {
			a.storeUpdatePreferences()
			if !u.prefs.AutomaticDownloads && u.phase == "downloading" && u.cancel != nil {
				u.cancel()
			} else if u.prefs.AutomaticDownloads && u.phase == "available" {
				a.downloadUpdate()
			}
		}
		ui.Text(c, a.tr("Checks every six hours. Installation always waits for your click.")).FontSize(12).TextColor(c.Theme().TextMuted)
		if !u.prefs.LastChecked.IsZero() {
			ui.Text(c, a.tr("Last checked")+": "+u.prefs.LastChecked.Local().Format("2006-01-02 15:04")).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		label := a.tr("Check for updates…")
		if u.busy || u.phase == "ready" || u.phase == "available" {
			label = a.updateStatus()
		}
		if ui.Button(c, label).AlignSelf(ui.Start).Clicked() {
			a.checkForUpdates()
		}
	})
}

func (a *application) updateNotice(c *ui.Context) {
	u := &a.updates
	if u.notice {
		u.notice = false
		action, perform := a.tr("View update"), a.showUpdateWindow
		if u.phase == "ready" {
			action, perform = a.tr("Install and Restart"), a.installUpdate
		}
		c.AddToast(ui.Toast{ID: "catview.update", Title: a.updateStatus(), Timeout: 12 * time.Second,
			Action: action, OnAction: perform})
	}
	if a.menu == nil {
		return
	}
	if item := a.menu.ItemByID("updates"); item != nil {
		label := a.tr("Check for updates…")
		if u.busy || u.phase == "ready" || u.phase == "available" {
			label = a.updateStatus()
		}
		if item.Label != label {
			item.SetLabel(label)
		}
	}
}
