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
		Title: a.tr("Settings"), Width: 620, Height: 520,
		TitleBarStyle: mygo.TitleBarHidden,
		MinWidth:      540, MinHeight: 480,
		StateKey: "settings",
		Content:  ui.View(a.settingsView),
	})
	a.settings = win
	win.OnFocus(func() { activeReader = a; a.installMenu() })
	win.OnClosed(func() {
		if a.settings == win {
			a.settings = nil
		}
	})
}

func (a *application) settingsChanged() {
	a.storePreferences()
	a.refreshAppearance()
}

func (a *application) refreshAppearance() {
	a.applySystemAppearance()
	if a.win != nil {
		a.win.SetVibrancy(a.sidebarMaterial())
		a.win.Invalidate()
	}
	a.epubSetFont()
	a.epubSetFamilies()
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
	// Preferences keep system colors; document palettes only color previews.
	t := *c.Theme()
	t.Spacing = 3
	if t.Dark {
		t.Background, t.Surface = ui.Hex("#252525"), ui.Hex("#303030")
		t.Border, t.Accent = ui.Hex("#474747"), ui.Hex("#0A84FF")
	} else {
		t.Background, t.Surface = ui.Hex("#F5F5F5"), ui.Hex("#FFFFFF")
		t.Border, t.Accent = ui.Hex("#DCDCDC"), ui.Hex("#007AFF")
	}
	c.SetTheme(&t)
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		ui.Row(c).Height(max(float32(28), c.TitleBar().Height)).FillWidth().DragWindow()
		a.settingsNavigation(c)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(620).AlignSelf(ui.Center).Padding(24, 32).Gap(24).Children(func() {
				switch a.settingsTab {
				case 0:
					a.appearanceSettings(c)
				case 1:
					a.readingSettings(c)
				case 2:
					a.translationSettingsView(c)
				case 3:
					a.updateSettings(c)
				}
				if a.preferencesError != "" {
					ui.Text(c, a.preferencesError).TextColor(t.Danger)
				}
			})
		})
	})
}

var settingsGlyphs = []*ui.SVG{
	ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8"/><path d="M12 4v16"/><path d="M12 4a8 8 0 0 1 0 16z" fill="currentColor" stroke="none"/></svg>`)),
	ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3 5h12M9 5v15M5 20h8M15 11h7M18.5 11v9M16 20h5"/></svg>`)),
	ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18M5 6h14M5 18h14"/></svg>`)),
	ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7a9 9 0 0 0-15-2L2 8m0-5v5h5M4 17a9 9 0 0 0 15 2l3-3m0 5v-5h-5"/></svg>`)),
}

func (a *application) settingsNavigation(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(8, 24, 12, 24).Justify(ui.Center).Children(func() {
		labels := []string{a.tr("Appearance"), a.tr("Reading"), a.tr("Translation"), a.tr("Updates")}
		tabs := ui.TabsBase(c, &a.settingsTab, len(labels))
		tabs.List.Gap(6).Label(a.tr("Settings")).Children(func() {
			for i, label := range labels {
				tab := tabs.Tab(i).MinWidth(72).Height(62).Padding(6, 12).Radius(8).Label(label)
				color := t.TextMuted
				if i == a.settingsTab {
					color = t.Accent
					tab.Background(t.Surface).Border(1, t.Border)
				} else if tab.Hovered() {
					tab.Background(t.SurfaceHover)
				}
				tab.Children(func() {
					ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(4).Children(func() {
						ui.Icon(c, settingsGlyphs[i]).FontSize(26).TextColor(color)
						ui.Text(c, label).FontSize(12).TextColor(color)
					})
				})
			}
		})
	})
}

func (a *application) appearanceSettings(c *ui.Context) {
	t := c.Theme()
	ui.Form(c, func() {
		ui.Field(c, a.tr("Appearance"), func() {
			if ui.Segmented(c, &a.themeMode, a.tr("System"), a.tr("Light"), a.tr("Dark")).AlignSelf(ui.Start).Label(a.tr("Appearance")).Changed() {
				a.settingsChanged()
			}
		})
		ui.Field(c, a.tr("Color theme"), func() {
			selected := a.tr(a.selectedTheme().name)
			var names []string
			for _, theme := range colorThemes {
				names = append(names, a.tr(theme.name))
			}
			if ui.Select(c, &selected, names).Width(240).AlignSelf(ui.Start).Label(a.tr("Color theme")).Changed() {
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
			picker.Track.AlignItems(ui.Center).Gap(5).Label(a.tr("Accent color")).Children(func() {
				for i, palette := range accentPalettes {
					chip := picker.Segment(i).Size(34, 34).Padding(4).Center().Radius(17).Label(a.tr(palette.name)).Tooltip(a.tr(palette.name))
					chip.Children(func() {
						ui.Box(c).Size(24, 24).Radius(12).Center().Background(ui.Hex(palette.swatch)).Children(func() {
							if i == a.accentIndex {
								color := ui.Hex("#FFFFFF")
								if i == 4 || i == 7 {
									color = ui.Hex("#1D1D1F")
								}
								ui.Text(c, "✓").FontSize(14).Bold().TextColor(color)
							}
						})
					})
				}
			})
			if picker.Track.Changed() {
				a.settingsChanged()
			}
		})
		a.languageSettings(c)
		ui.Field(c, a.tr("File icons"), func() {
			if ui.Segmented(c, &a.fileIconStyle, a.tr("System"), "Material").AlignSelf(ui.Start).Label(a.tr("File icons")).Changed() {
				a.systemIcons = nil
				a.settingsChanged()
			}
		})
	})
	// Reading colors are confined to this live sample.
	t = a.theme(c)
	ui.Row(c).Height(96).FillWidth().Background(t.Background).TextColor(t.Text).Border(1, t.Border).Radius(6).Children(func() {
		ui.Column(c).Width(150).FillHeight().Padding(12).Gap(6).Background(t.Surface).Children(func() {
			ui.Text(c, "CatView").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "README.md").FontSize(12).FillWidth().Padding(5, 7).Radius(4).Background(t.Accent).TextColor(t.AccentText)
		})
		ui.Column(c).Grow(1).Padding(14, 18).Gap(6).Children(func() {
			ui.Text(c, a.tr("Open and read.")).Bold()
			ui.Text(c, a.tr("Theme preview")).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, a.tr("Link")).FontSize(12).TextColor(a.linkColor(t.Dark))
		})
	})
}

func (a *application) readingSettings(c *ui.Context) {
	a.fontSettings(c)
	ui.Form(c, func() {
		ui.Field(c, a.tr("Font size"), func() {
			fontSize := float64(a.fontSize)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.StepSlider(c, &fontSize, float64(minReadingFontSize), float64(maxReadingFontSize), 1).Label(a.tr("Font size")).Grow(1).Changed() {
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
			if ui.Segmented(c, &a.readingWidth, a.tr("Standard"), a.tr("Wide")).AlignSelf(ui.Start).Label(a.tr("Text width")).Changed() {
				a.settingsChanged()
			}
		})
	})
	t := a.theme(c)
	ui.Column(c).FillWidth().Padding(18).Gap(a.fontSize*(a.lineHeight-1)).Background(t.Background).TextColor(t.Text).Border(1, t.Border).Radius(6).Children(func() {
		ui.Text(c, a.tr("Open and read.")).Font(a.readingFont).FontSize(a.fontSize).Bold()
		ui.Text(c, a.tr("Reading preview")).Font(a.readingFont).FontSize(a.fontSize).TextColor(t.TextMuted)
	})
}

func (a *application) preferredTextWidth() float32 {
	if a.readingWidth == 1 {
		return 1000
	}
	return 800
}
