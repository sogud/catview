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
		Title: a.tr("Settings"), Width: 600, Height: 720,
		MinWidth: 540, MinHeight: 450,
		StateKey: "settings",
		Content:  ui.View(a.settingsView),
	})
	a.settings = win
	win.OnClosed(func() {
		if a.settings == win {
			a.settings = nil
		}
	})
}

func (a *application) settingsChanged() {
	a.storePreferences()
	if a.win != nil {
		a.win.Invalidate()
	}
	a.epubSetFont()
	a.epubSetTheme()
	a.epubSetLayout()
	a.epubSetAccent()
	a.updateHTMLPreview()
}

func (a *application) settingsView(c *ui.Context) {
	c.SetTheme(a.theme(c))
	t := c.Theme()
	ui.Scroll(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).FillWidth().Padding(25, 30).Gap(16).Children(func() {
			ui.Form(c, func() { a.languageSettings(c) })
			ui.Text(c, a.tr("Appearance")).FontSize(15).Bold()
			ui.Form(c, func() {
				ui.Field(c, a.tr("Appearance"), func() {
					if ui.Segmented(c, &a.themeMode, a.tr("System"), a.tr("Light"), a.tr("Dark")).Label(a.tr("Appearance")).Changed() {
						a.settingsChanged()
					}
				})
				ui.Field(c, a.tr("File icons"), func() {
					if ui.Segmented(c, &a.fileIconStyle, a.tr("System"), "Material").Label(a.tr("File icons")).Changed() {
						a.systemIcons = nil
						a.settingsChanged()
					}
				})
				ui.Field(c, a.tr("Accent color"), func() {
					ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
						picker := ui.SegmentedBase(c, &a.accentIndex, len(accentPalettes))
						picker.Track.AlignItems(ui.Center).Gap(6).Label(a.tr("Accent color")).Children(func() {
							for i, palette := range accentPalettes {
								chip := picker.Segment(i).Size(38, 32).Radius(8).Label(a.tr(palette.name)).Tooltip(a.tr(palette.name))
								if i == a.accentIndex {
									chip.Background(t.SurfaceHover).Border(1, t.TextMuted)
								} else if chip.Hovered() {
									chip.Background(t.SurfaceHover)
								}
								chip.Children(func() {
									ui.Box(c).Size(17, 17).Radius(9).Background(ui.Hex(palette.swatch))
								})
							}
						})
						if picker.Track.Changed() {
							a.settingsChanged()
						}
						ui.Text(c, a.tr(a.palette().name)).FontSize(12).TextColor(t.TextMuted)
					})
				})
			})
			ui.Divider(c)
			ui.Text(c, a.tr("Reading")).FontSize(15).Bold()
			ui.Form(c, func() {
				ui.Field(c, a.tr("Font size"), func() {
					fontSize := float64(a.fontSize)
					ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
						if ui.StepSlider(c, &fontSize, 12, 24, 1).Label(a.tr("Font size")).Grow(1).Changed() {
							a.fontSize = float32(fontSize)
							a.settingsChanged()
						}
						ui.Text(c, fmt.Sprintf("%.0f", a.fontSize)).Width(30).TextAlign(ui.End)
					})
				})
				ui.Field(c, a.tr("Line spacing"), func() {
					spacing := float64(a.lineHeight)
					ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
						if ui.StepSlider(c, &spacing, 1.4, 2.0, 0.1).Label(a.tr("Line spacing")).Grow(1).Changed() {
							a.lineHeight = float32(spacing)
							a.settingsChanged()
						}
						ui.Text(c, fmt.Sprintf("%.1f", a.lineHeight)).Width(30).TextAlign(ui.End)
					})
				})
				ui.Field(c, a.tr("Text width"), func() {
					if ui.Segmented(c, &a.readingWidth, a.tr("Standard"), a.tr("Wide")).Label(a.tr("Text width")).Changed() {
						a.settingsChanged()
					}
				})
			})
			ui.Divider(c)
			a.translationSettingsView(c)
		})
	})
}

func (a *application) preferredTextWidth() float32 {
	if a.readingWidth == 1 {
		return 1000
	}
	return 800
}
