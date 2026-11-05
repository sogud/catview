package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	translatekit "github.com/sogud/translate-kit"
)

// Credentials stay in CatView's local application data; never in Keychain.
type translationSettings struct {
	Provider string `json:"provider"`
	Target   string `json:"target"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	Key      string `json:"api_key,omitempty"`
}
type documentTranslation struct {
	cancel   context.CancelFunc
	serial   uint64
	busy     bool
	progress string
	text     string
	revision string
	mode     int // 0 original, 1 translated, 2 bilingual
	md       markdown.Options
}

func (s translationSettings) config() translatekit.Config {
	return translatekit.Config{Provider: s.Provider, TargetLanguage: s.Target, BaseURL: s.Endpoint, LibreURL: s.Endpoint, Model: s.Model}
}
func (a *application) translationSettingsPath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "translation.json")
}
func (a *application) loadTranslationSettings() {
	a.translationConfig = translationSettings{Provider: "ai", Target: "zh-CN"}
	path := a.translationSettingsPath()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var settings translationSettings
	if json.Unmarshal(data, &settings) == nil {
		a.translationConfig = settings
	}
}
func (a *application) saveTranslationSettings() {
	s := a.translationConfig
	if err := translatekit.Validate(s.config()); err != nil {
		a.translationNotice = err.Error()
		return
	}
	path := a.translationSettingsPath()
	if path == "" {
		a.translationNotice = a.tr("Could not save settings")
		return
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		a.translationNotice = a.tr("Could not save settings")
		return
	} else if err != nil && !os.IsNotExist(err) {
		a.translationNotice = a.tr("Could not save settings")
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		a.translationNotice = a.tr("Could not save settings")
		return
	}
	// CreateTemp uses 0600. Rename avoids a partially written key file.
	file, err := os.CreateTemp(filepath.Dir(path), ".translation-*")
	if err != nil {
		a.translationNotice = a.tr("Could not save settings")
		return
	}
	name := file.Name()
	defer os.Remove(name)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		a.translationNotice = a.tr("Could not save settings")
		return
	}
	a.translationNotice = a.tr("Saved locally")
}
func (a *application) translationSettingsView(c *ui.Context) {
	ui.Form(c, func() {
		ui.Field(c, a.tr("Provider"), func() {
			ui.Select(c, &a.translationConfig.Provider, []string{"ai", "google", "google-cloud", "youdao", "libretranslate"}).Width(240).AlignSelf(ui.Start).Radius(12).Material(glass.Glass{Interactive: true}).Label(a.tr("Provider"))
		})
		ui.Field(c, a.tr("Target language"), func() {
			ui.Select(c, &a.translationConfig.Target, []string{"zh-CN", "zh-TW", "en", "ja", "ko", "fr", "de", "es", "pt", "ru"}).Width(240).AlignSelf(ui.Start).Radius(12).Material(glass.Glass{Interactive: true}).Label(a.tr("Target language"))
		})
		ui.Field(c, a.tr("Endpoint"), func() { ui.TextInput(c, &a.translationConfig.Endpoint).Label(a.tr("Endpoint")) })
		ui.Field(c, a.tr("Model"), func() { ui.TextInput(c, &a.translationConfig.Model).Label(a.tr("Model")) })
		ui.Field(c, a.tr("API key"), func() { ui.TextInput(c, &a.translationConfig.Key).Password().Label(a.tr("API key")) })
	})
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, a.tr("Only explicit translation sends text to the selected provider. Google and Youdao web services are unofficial.")).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, a.tr("Keys are saved locally as plain text with user-only file permissions, not in Keychain.")).FontSize(11).TextColor(c.Theme().TextMuted)
	})
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.translationNotice).Grow(1).FontSize(11).TextColor(c.Theme().TextMuted)
		if ui.Button(c, a.tr("Save")).MinWidth(76).Height(30).Radius(15).Material(glass.Glass{Interactive: true}).Clicked() {
			a.saveTranslationSettings()
		}
	})
}
func canTranslate(d *document) bool {
	if d == nil || d.editing || documentModified(d) {
		return false
	}
	if d.kind == kindMarkdown {
		return true
	}
	ext := strings.ToLower(filepath.Ext(d.path))
	return d.kind == kindText && (ext == ".txt" || ext == ".text")
}
func translationRevision(d *document) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(d.source)))
}
func cancelTranslation(d *document) {
	if d == nil {
		return
	}
	t := &d.translation
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	t.serial++
	t.busy = false
	t.progress = ""
}
func clearTranslation(d *document) {
	cancelTranslation(d)
	d.translation.text = ""
	d.translation.revision = ""
	d.translation.mode = 0
}
func (a *application) translationCurrent(d *document, serial uint64, revision string) bool {
	if a.win == nil || a.win.IsDestroyed() || d.translation.serial != serial || d.editing || translationRevision(d) != revision {
		return false
	}
	for _, tab := range a.tabs {
		if tab == d {
			return true
		}
	}
	return false
}
func (a *application) requestTranslation() {
	d := a.doc
	if !canTranslate(d) {
		return
	}
	settings := a.translationConfig // Includes key only in memory; never log this value.
	if err := translatekit.Validate(settings.config()); err != nil {
		a.translationNotice = err.Error()
		a.showSettings()
		return
	}
	if len(d.source) > 1024*1024 {
		a.translationMessage(a.tr("Translate a document smaller than 1 MiB"))
		return
	}
	cancelTranslation(d)
	serial := d.translation.serial
	revision := translationRevision(d)
	body := d.source
	provider := settings.Provider
	if provider == "ai" || provider == "libretranslate" {
		endpoint, _ := url.Parse(settings.Endpoint)
		provider += " · " + endpoint.Host
	}
	win := a.win
	message := a.tr("Send this document to the selected translation provider?") + "\n" + provider + " → " + settings.Target + "\n" + a.tr("The original file will not change.")
	sendLabel, cancelLabel := a.tr("Translate document"), a.tr("Cancel")
	go func() {
		result, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageWarning, Message: message, Buttons: []string{sendLabel, cancelLabel}, DefaultButton: 1, CancelButton: 1})
		win.Update(func() {
			if err != nil || result.Button != 0 || !a.translationCurrent(d, serial, revision) {
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			d.translation.text = ""
			d.translation.revision = ""
			d.translation.mode = 0
			d.translation.cancel = cancel
			d.translation.busy = true
			d.translation.progress = ""
			if a.translator == nil {
				a.translator = translatekit.New(translatekit.Options{})
			}
			client := a.translator
			go func() {
				defer cancel()
				translated, err := client.Translate(ctx, translatekit.Request{Config: settings.config(), APIKey: settings.Key, Text: body, Revision: revision}, func(p translatekit.Progress) {
					win.Update(func() {
						if a.translationCurrent(d, serial, revision) {
							d.translation.progress = fmt.Sprintf("%d / %d", p.Completed, p.Total)
							win.Invalidate()
						}
					})
				})
				win.Update(func() {
					if !a.translationCurrent(d, serial, revision) {
						return
					}
					d.translation.cancel = nil
					d.translation.busy = false
					d.translation.progress = ""
					if err != nil {
						// Core redacts provider errors; the UI deliberately uses a generic message.
						if ctx.Err() == nil {
							a.translationMessage(a.tr("Translation failed. Check provider settings or try again."))
						}
						win.Invalidate()
						return
					}
					if translated.Revision != revision {
						return
					}
					d.translation.text = translated.Text
					d.translation.revision = revision
					d.translation.mode = 1
					win.Invalidate()
				})
			}()
			win.Invalidate()
		})
	}()
}
func (a *application) translationMessage(message string) {
	win := a.win
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageWarning, Message: message, Buttons: []string{"OK"}})
	}()
}
func (a *application) translationMenuItems() []*mygo.MenuItem {
	action := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { fn(); a.win.Invalidate() }
	}
	items := []*mygo.MenuItem{
		{ID: "translate", Label: a.tr("Translate document"), Click: action(a.requestTranslation)},
		{ID: "translation-cancel", Label: a.tr("Cancel translation"), Click: action(func() { cancelTranslation(a.doc) })},
		mygo.Separator(),
	}
	for mode, key := range []string{"Original", "Translated", "Bilingual"} {
		value := mode
		items = append(items, &mygo.MenuItem{ID: fmt.Sprintf("translation-mode-%d", mode), Label: a.tr(key), Type: mygo.MenuItemCheckbox, Click: action(func() {
			if a.doc != nil {
				a.doc.translation.mode = value
			}
		})})
	}
	return items
}
func (a *application) syncTranslationMenu() {
	if a.menu == nil {
		return
	}
	d := a.doc
	busy := d != nil && d.translation.busy
	hasResult := d != nil && d.translation.text != "" && d.translation.revision == translationRevision(d) && !d.editing
	for id, enabled := range map[string]bool{"translate": canTranslate(d) && !busy, "translation-cancel": busy} {
		if item := a.menu.ItemByID(id); item != nil && item.IsEnabled() != enabled {
			item.SetEnabled(enabled)
		}
	}
	for mode := 0; mode < 3; mode++ {
		if item := a.menu.ItemByID(fmt.Sprintf("translation-mode-%d", mode)); item != nil {
			enabled := d != nil && (mode == 0 || hasResult)
			if item.IsEnabled() != enabled {
				item.SetEnabled(enabled)
			}
			checked := d != nil && d.translation.mode == mode
			if item.IsChecked() != checked {
				item.SetChecked(checked)
			}
		}
	}
}
func (a *application) translationReader(c *ui.Context, d *document) bool {
	state := &d.translation
	if state.busy {
		ui.Row(c).Padding(6, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Translating…")+" "+state.progress).FontSize(12).Grow(1)
			if ui.Button(c, a.tr("Cancel")).Clicked() {
				cancelTranslation(d)
			}
		})
	}
	if state.mode == 0 || state.text == "" || d.editing || state.revision != translationRevision(d) {
		return false
	}
	state.md.LoadImage = d.md.LoadImage
	state.md.Theme = &markdown.Theme{FontSize: a.fontSize, LineHeight: a.lineHeight, Link: a.linkColor(c.Theme().Dark), BlockSpacing: 1.1, CodeSize: 1, CodeBlockBackground: c.Theme().Background}
	ui.Scroll(c).Grow(1).Padding(30, 38).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(a.preferredTextWidth()).AlignSelf(ui.Center).Gap(20).Children(func() {
			if state.mode == 2 {
				ui.Text(c, a.tr("Original")).Bold()
				// Separate options prevent original and translated render state collision.
				if d.kind == kindMarkdown {
					d.md.Theme = state.md.Theme
					markdown.View(c, d.source, &d.md)
				} else {
					ui.Text(c, d.source)
				}
				ui.Divider(c)
				ui.Text(c, a.tr("Translated")).Bold()
			}
			if d.kind == kindMarkdown {
				markdown.View(c, state.text, &state.md)
			} else {
				ui.Text(c, state.text)
			}
		})
	})
	return true
}
