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
		Title: "设置", Width: 520, Height: 425,
		MinWidth: 500, MinHeight: 405,
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
	ui.Column(c).Fill().Padding(25, 30).Gap(16).Background(t.Background).Children(func() {
		ui.Text(c, "外观").FontSize(15).Bold()
		ui.Form(c, func() {
			ui.Field(c, "模式", func() {
				if ui.Segmented(c, &a.themeMode, "系统", "浅色", "深色").Label("外观模式").Changed() {
					a.settingsChanged()
				}
			})
			ui.Field(c, "强调色", func() {
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					picker := ui.SegmentedBase(c, &a.accentIndex, len(accentPalettes))
					picker.Track.AlignItems(ui.Center).Gap(6).Label("强调色").Children(func() {
						for i, palette := range accentPalettes {
							chip := picker.Segment(i).Size(38, 32).Radius(8).Label(palette.name).Tooltip(palette.name)
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
					ui.Text(c, a.palette().name).FontSize(12).TextColor(t.TextMuted)
				})
			})
		})
		ui.Divider(c)
		ui.Text(c, "阅读").FontSize(15).Bold()
		ui.Form(c, func() {
			ui.Field(c, "字号", func() {
				fontSize := float64(a.fontSize)
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					if ui.StepSlider(c, &fontSize, 12, 24, 1).Label("阅读字号").Grow(1).Changed() {
						a.fontSize = float32(fontSize)
						a.settingsChanged()
					}
					ui.Text(c, fmt.Sprintf("%.0f", a.fontSize)).Width(30).TextAlign(ui.End)
				})
			})
			ui.Field(c, "行距", func() {
				spacing := float64(a.lineHeight)
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					if ui.StepSlider(c, &spacing, 1.4, 2.0, 0.1).Label("阅读行距").Grow(1).Changed() {
						a.lineHeight = float32(spacing)
						a.settingsChanged()
					}
					ui.Text(c, fmt.Sprintf("%.1f", a.lineHeight)).Width(30).TextAlign(ui.End)
				})
			})
			ui.Field(c, "正文宽度", func() {
				if ui.Segmented(c, &a.readingWidth, "标准", "宽屏").Label("正文宽度").Changed() {
					a.settingsChanged()
				}
			})
		})
	})
}

func (a *application) preferredTextWidth() float32 {
	if a.readingWidth == 1 {
		return 1000
	}
	return 800
}
