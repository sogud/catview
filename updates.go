package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// Keep runtime update comparisons aligned with the packaged release version.
//
//go:embed mygo.json
var packagedConfig []byte

var releaseVersion = func() string {
	var config struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(packagedConfig, &config); err != nil {
		panic(err)
	}
	return config.Version
}()

type updatePreferences struct {
	AutomaticChecks    bool      `json:"automatic_checks"`
	AutomaticDownloads bool      `json:"automatic_downloads"`
	LastChecked        time.Time `json:"last_checked,omitempty"`
	NextCheck          time.Time `json:"next_check,omitempty"`
	RetryAfter         time.Time `json:"retry_after,omitempty"`
}

type updateController struct {
	prefs             updatePreferences
	window            *mygo.Window
	phase, detail     string
	busy, notice      bool
	downloaded, total int64
	update            *mygo.Update
	source            *preparedUpdate
	cacheDir          string
	cancel, stop      context.CancelFunc
}

func (a *application) startUpdates() {
	u := a.updates
	u.prefs = updatePreferences{AutomaticChecks: true, AutomaticDownloads: true}
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, "updates.json")); err == nil {
			_ = json.Unmarshal(data, &u.prefs)
		}
	}
	if dir, err := mygo.App.Path(mygo.PathCache); err == nil {
		u.cacheDir = filepath.Join(dir, "updates")
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.stop = cancel
	go func() {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				a.win.Update(func() {
					if updateOwner != a || a.win.IsDestroyed() {
						return
					}

					// A downloaded update remains available after restarting offline.
					_, pending := os.Stat(filepath.Join(u.cacheDir, "pending.json"))
					if u.prefs.AutomaticChecks && (pending == nil || !time.Now().Before(u.prefs.NextCheck)) {
						a.beginUpdateCheck()
					}
				})
				timer.Reset(time.Hour)
			}
		}
	}()
}

func (a *application) stopUpdates() {
	u := a.updates
	if u.stop != nil {
		u.stop()
	}
	if u.cancel != nil {
		u.cancel()
	}
	if u.window != nil {
		u.window.Close()
	}
}

func (a *application) storeUpdatePreferences() {
	if updateOwner != nil && updateOwner != a {
		updateOwner.storeUpdatePreferences()
		return
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err == nil {
		var data []byte
		data, err = json.Marshal(a.updates.prefs)
		if err == nil {
			err = writePreferences(filepath.Join(dir, "updates.json"), data)
		}
	}
	if err != nil {
		a.updates.detail = a.tr("Could not save settings") + ": " + err.Error()
	}
}

func (a *application) refreshUpdateUI() {
	if a.updates.window != nil {
		a.updates.window.Invalidate()
	}
	if a.settings != nil {
		a.settings.Invalidate()
	}
	for _, reader := range readerWindows {
		reader.win.Invalidate()
		if reader.settings != nil {
			reader.settings.Invalidate()
		}
	}
}

func (a *application) checkForUpdates() {
	if updateOwner != nil && updateOwner != a {
		updateOwner.checkForUpdates()
		return
	}
	a.showUpdateWindow()
	a.beginUpdateCheck()
}

func (a *application) updateError(err error) string {
	var response *updateHTTPError
	if errors.As(err, &response) && (response.status == 403 || response.status == 429) {
		return fmt.Sprintf(a.tr("GitHub is refusing requests (HTTP %d). Retry later or open the download page."), response.status)
	}
	var connection *url.Error
	if errors.As(err, &connection) || errors.Is(err, context.DeadlineExceeded) {
		return a.tr("Could not connect to GitHub. Check your connection or retry later.")
	}
	return a.tr(err.Error())
}

func (a *application) beginUpdateCheck() {
	if updateOwner != nil && updateOwner != a {
		updateOwner.beginUpdateCheck()
		return
	}
	u := a.updates
	if u.busy || u.phase == "ready" || u.phase == "available" {
		return
	}
	if time.Now().Before(u.prefs.RetryAfter) {
		u.phase, u.detail = "error", fmt.Sprintf(a.tr("GitHub is temporarily limiting requests. Try again after %s."), u.prefs.RetryAfter.Local().Format("15:04"))
		a.refreshUpdateUI()
		return
	}
	u.phase, u.detail, u.busy = "checking", "", true
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	u.cancel = cancel
	a.refreshUpdateUI()
	cacheDir := u.cacheDir
	go func() {
		defer cancel()
		source, err := findUpdate(ctx, cacheDir)
		var update *mygo.Update
		cached := false
		if err == nil && source != nil {
			mygo.Updater.SetSource(source)
			update, err = mygo.Updater.Check(ctx)
			cached = source.cached() == nil
		}
		a.win.Update(func() {
			if updateOwner != a || a.win.IsDestroyed() {
				return
			}

			u.busy, u.cancel = false, nil
			u.prefs.NextCheck = time.Now().Add(6 * time.Hour)
			if err != nil {
				u.phase, u.detail = "error", a.updateError(err)
				if errors.Is(err, context.Canceled) {
					u.phase, u.detail = "idle", ""
				}
				if errors.Is(err, mygo.ErrUpdatesDisabled) {
					u.detail = a.tr("Move CatView to a writable Applications folder to install updates.")
				}
				u.prefs.NextCheck = time.Now().Add(time.Hour)
				var response *updateHTTPError
				if errors.As(err, &response) && time.Now().Before(response.retryAfter) {
					u.prefs.RetryAfter = response.retryAfter
					u.prefs.NextCheck = response.retryAfter
					u.detail = fmt.Sprintf(a.tr("GitHub is temporarily limiting requests. Try again after %s."), response.retryAfter.Local().Format("15:04"))
				}
			} else {
				u.prefs.LastChecked = time.Now()
				u.prefs.RetryAfter = time.Time{}
				u.source, u.update = source, update
				u.phase = "current"
				if update != nil {
					u.phase = "available"
					if cached || u.prefs.AutomaticDownloads {
						a.downloadUpdate()
					} else {
						u.notice = true
					}
				}
			}
			a.storeUpdatePreferences()
			a.refreshUpdateUI()
		})
	}()
}

func (a *application) downloadUpdate() {
	if updateOwner != nil && updateOwner != a {
		updateOwner.downloadUpdate()
		return
	}
	u := a.updates
	if u.busy || u.source == nil || u.update == nil {
		return
	}
	u.phase, u.detail, u.busy = "downloading", "", true
	u.downloaded, u.total = 0, u.source.manifest.Size
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	u.cancel = cancel
	source := u.source
	a.refreshUpdateUI()
	go func() {
		defer cancel()
		lastProgress := time.Time{}
		err := source.download(ctx, func(n int64) {
			if time.Since(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = time.Now()
			a.win.Update(func() {
				if updateOwner != a || a.win.IsDestroyed() {
					return
				}
				u.downloaded = n
				a.refreshUpdateUI()
			})
		})
		a.win.Update(func() {
			if updateOwner != a || a.win.IsDestroyed() {
				return
			}

			u.busy, u.cancel = false, nil
			if errors.Is(err, context.Canceled) {
				u.phase, u.detail = "available", a.tr("Download paused. You can try again later.")
			} else if err != nil {
				u.phase, u.detail = "available", a.updateError(err)
			} else {
				u.phase, u.notice = "ready", true
				u.downloaded = u.total
			}
			a.refreshUpdateUI()
		})
	}()
}

func (a *application) installUpdate() {
	if updateOwner != nil && updateOwner != a {
		updateOwner.installUpdate()
		return
	}
	u := a.updates
	if u.busy || u.phase != "ready" || u.update == nil {
		return
	}
	a.showUpdateWindow()
	for _, reader := range readerWindows {
		if reader.fileOperation || reader.closePrompt {
			u.detail = a.tr("Finish the current file operation before installing.")
			a.refreshUpdateUI()
			return
		}
		for _, d := range reader.tabs {
			if documentModified(d) || d.saving {
				u.detail = a.tr("Save or discard unsaved changes before installing the update.")
				reader.win.Focus()
				a.refreshUpdateUI()
				return
			}
		}
	}
	// Existing file-operation guards also make all editors read-only while
	// the verified bundle is staged. No helper is started before this click.
	for _, reader := range readerWindows {
		reader.fileOperation = true
	}
	u.phase, u.detail, u.busy = "installing", "", true
	update, source := u.update, u.source
	a.refreshUpdateUI()
	go func() {
		err := update.Install(context.Background(), nil)
		invalidCache := err != nil && source.cached() != nil
		a.win.Update(func() {
			if updateOwner != a || a.win.IsDestroyed() {
				return
			}

			for _, reader := range readerWindows {
				reader.fileOperation = false
			}
			u.busy = false
			u.phase = "ready"
			if err != nil {
				u.detail = a.tr(err.Error())
				if invalidCache {
					u.phase = "available"
					u.detail = a.tr("The downloaded update is missing or damaged. Download it again.")
				}
				a.refreshUpdateUI()
				return
			}
			mygo.App.Relaunch()
		})
	}()
}

func parseVersion(version string) ([3]uint64, []string, bool) {
	var numbers [3]uint64
	if core, metadata, hasMetadata := strings.Cut(version, "+"); hasMetadata {
		for _, part := range strings.Split(metadata, ".") {
			if part == "" {
				return numbers, nil, false
			}
			for _, ch := range part {
				if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '-') {
					return numbers, nil, false
				}
			}
		}
		version = core
	}
	parts := strings.SplitN(version, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return numbers, nil, false
	}
	for i, part := range core {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || (len(part) > 1 && part[0] == '0') {
			return numbers, nil, false
		}
		numbers[i] = value
	}
	var prerelease []string
	if len(parts) == 2 {
		prerelease = strings.Split(parts[1], ".")
		for _, part := range prerelease {
			if part == "" {
				return numbers, nil, false
			}
			if numericIdentifier(part) && len(part) > 1 && part[0] == '0' {
				return numbers, nil, false
			}
			for _, ch := range part {
				if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '-') {
					return numbers, nil, false
				}
			}
		}
	}
	return numbers, prerelease, true
}

func newerVersion(candidate, current string) bool {
	a, ap, aok := parseVersion(candidate)
	b, bp, bok := parseVersion(current)
	if !aok || !bok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	if len(ap) == 0 || len(bp) == 0 {
		return len(ap) == 0 && len(bp) != 0
	}
	for i := 0; i < min(len(ap), len(bp)); i++ {
		if ap[i] == bp[i] {
			continue
		}
		an, bn := numericIdentifier(ap[i]), numericIdentifier(bp[i])
		if an && bn && len(ap[i]) != len(bp[i]) {
			return len(ap[i]) > len(bp[i])
		}
		if an != bn {
			return !an
		}
		return ap[i] > bp[i]
	}
	return len(ap) > len(bp)
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
