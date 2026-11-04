package main

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Settings uses a separate native macOS window. The reading window remains
// unchanged: one file tree and one content surface.
func (a *application) showSettings() {
	if a.settings != nil && !a.settings.IsDestroyed() {
		a.settings.Show()
		a.settings.Focus()
		return
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: a.tr("Settings"), Width: 620, Height: a.settingsHeight(),
		MinWidth: 540, MinHeight: 340,
		StateKey: "settings",
		Content:  ui.View(a.settingsView),
	})
	a.settings = win
	width, _ := win.Size()
	win.SetSize(width, a.settingsHeight())
	win.OnClosed(func() {
		if a.settings == win {
			a.settings = nil
		}
	})
}

func (a *application) settingsHeight() int {
	switch a.settingsTab {
	case 1:
		return 360
	case 2:
		return 480
	default:
		return 460
	}
}

func (a *application) settingsChanged() {
	a.applySystemAppearance()
	a.storePreferences()
	if a.win != nil {
		a.win.Invalidate()
	}
	a.epubSetFont()
	a.epubSetTheme()
	a.epubSetLayout()
	a.epubSetAccent()
	a.updateHTMLPreview()
	for _, view := range []*mygo.WebView{a.htmlView, a.epubView, a.pdf} {
		if view != nil {
			_ = view.SetBackgroundColor(a.webBackground())
		}
	}
}

func (a *application) settingsView(c *ui.Context) {
	c.SetTheme(a.theme(c))
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		ui.Row(c).Padding(12, 24).Justify(ui.Center).Children(func() {
			if ui.Segmented(c, &a.settingsTab, a.tr("Appearance"), a.tr("Reading"), a.tr("Translation")).Label(a.tr("Settings")).Changed() && a.settings != nil {
				width, _ := a.settings.Size()
				a.settings.SetSize(width, a.settingsHeight())
			}
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(20, 24).Gap(20).Children(func() {
				switch a.settingsTab {
				case 0:
					a.appearanceSettings(c)
				case 1:
					a.readingSettings(c)
				case 2:
					a.translationSettingsView(c)
				}
				if a.preferencesError != "" {
					ui.Text(c, a.preferencesError).TextColor(t.Danger)
				}
			})
		})
	})
}

func (a *application) appearanceSettings(c *ui.Context) {
	t := c.Theme()
	ui.Form(c, func() {
		ui.Field(c, a.tr("Appearance"), func() {
			if ui.Segmented(c, &a.themeMode, a.tr("System"), a.tr("Light"), a.tr("Dark")).Label(a.tr("Appearance")).Changed() {
				a.settingsChanged()
			}
		})
		ui.Field(c, a.tr("Color theme"), func() {
			selected := a.tr(a.selectedTheme().name)
			var names []string
			for _, theme := range colorThemes {
				names = append(names, a.tr(theme.name))
			}
			if ui.Select(c, &selected, names).Label(a.tr("Color theme")).Changed() {
				for _, theme := range colorThemes {
					if a.tr(theme.name) == selected {
						a.colorTheme = theme.id
						a.accentIndex = theme.accent
						break
					}
				}
				a.settingsChanged()
			}
		})
		ui.Field(c, a.tr("Accent color"), func() {
			picker := ui.SegmentedBase(c, &a.accentIndex, len(accentPalettes))
			picker.Track.AlignItems(ui.Center).Gap(3).Label(a.tr("Accent color")).Children(func() {
				for i, palette := range accentPalettes {
					chip := picker.Segment(i).Size(32, 30).Radius(6).Label(a.tr(palette.name)).Tooltip(a.tr(palette.name))
					if i == a.accentIndex {
						chip.Background(t.SurfaceHover).Border(1, t.TextMuted)
					}
					chip.Children(func() { ui.Box(c).Size(16, 16).Radius(8).Background(ui.Hex(palette.swatch)) })
				}
			})
			if picker.Track.Changed() {
				a.settingsChanged()
			}
		})
		a.languageSettings(c)
		ui.Field(c, a.tr("File icons"), func() {
			if ui.Segmented(c, &a.fileIconStyle, a.tr("System"), "Material").Label(a.tr("File icons")).Changed() {
				a.systemIcons = nil
				a.settingsChanged()
			}
		})
	})
	// A small live sample exposes text, selection and link contrast together.
	ui.Row(c).Height(96).FillWidth().Border(1, t.Border).Radius(6).Children(func() {
		ui.Column(c).Width(150).FillHeight().Padding(12).Gap(6).Background(t.Surface).Children(func() {
			ui.Text(c, "CatView").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "README.md").FontSize(12).FillWidth().Padding(5, 7).Radius(4).Background(t.Accent)
		})
		ui.Column(c).Grow(1).Padding(14, 18).Gap(6).Children(func() {
			ui.Text(c, a.tr("Open and read.")).Bold()
			ui.Text(c, a.tr("Theme preview")).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, a.tr("Link")).FontSize(12).TextColor(a.linkColor(t.Dark))
		})
	})
}

func (a *application) readingSettings(c *ui.Context) {
	ui.Form(c, func() {
		ui.Field(c, a.tr("Font size"), func() {
			fontSize := float64(a.fontSize)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.StepSlider(c, &fontSize, 12, 24, 1).Label(a.tr("Font size")).Grow(1).Changed() {
					a.fontSize = float32(fontSize)
					a.settingsChanged()
				}
				ui.Text(c, fmt.Sprintf("%.0f", a.fontSize)).Width(36).TextAlign(ui.End)
			})
		})
		ui.Field(c, a.tr("Line spacing"), func() {
			spacing := float64(a.lineHeight)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.StepSlider(c, &spacing, 1.4, 2.0, 0.05).Label(a.tr("Line spacing")).Grow(1).Changed() {
					a.lineHeight = float32(spacing)
					a.settingsChanged()
				}
				ui.Text(c, fmt.Sprintf("%.2f", a.lineHeight)).Width(36).TextAlign(ui.End)
			})
		})
		ui.Field(c, a.tr("Text width"), func() {
			if ui.Segmented(c, &a.readingWidth, a.tr("Standard"), a.tr("Wide")).Label(a.tr("Text width")).Changed() {
				a.settingsChanged()
			}
		})
	})
	ui.Divider(c)
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, a.tr("Open and read.")).FontSize(a.fontSize).Bold()
		ui.Text(c, a.tr("Reading preview")).FontSize(a.fontSize).TextColor(c.Theme().TextMuted)
	})
}

func (a *application) preferredTextWidth() float32 {
	if a.readingWidth == 1 {
		return 1000
	}
	return 800
}
