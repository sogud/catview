package main

import (
	_ "embed"
	"strings"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/mark.svg
var welcomeMarkSource string

// Reuse the app's existing cat mark as a quiet monochrome watermark.
var welcomeMark = ui.MustParseSVG([]byte(strings.NewReplacer(
	"#E7D4BD", "currentColor", "#F1CDBD", "currentColor", "#252C39", "currentColor",
).Replace(welcomeMarkSource)))

func (a *application) welcomeView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(28).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(320).AlignItems(ui.Center).Gap(28).Children(func() {
			ui.Icon(c, welcomeMark).FontSize(154).TextColor(t.TextMuted).Opacity(0.28)
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				a.welcomeAction(c, a.tr("Open file…"), []string{"⌘", "O"}, false)
				a.welcomeAction(c, a.tr("Open folder…"), []string{"⇧", "⌘", "O"}, true)
			})
		})
	})
}

func (a *application) welcomeAction(c *ui.Context, label string, shortcut []string, folder bool) {
	t := c.Theme()
	button := ui.ButtonBase(c).FillWidth().Height(42).Padding(8, 12).Radius(7).Label(label).Disabled(a.fileOperation)
	text := t.TextMuted
	if button.Hovered() || button.Focused() {
		button.Background(t.SurfaceHover)
		text = t.Text
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(14).TextColor(text).Grow(1).SingleLine()
		ui.Row(c).Gap(4).Children(func() {
			for _, key := range shortcut {
				ui.Box(c).Size(24, 24).Center().Radius(4).Background(t.Surface).Border(1, t.Border).Children(func() {
					ui.Text(c, key).FontSize(12).TextColor(text)
				})
			}
		})
	})
	if button.Clicked() {
		a.choose(folder)
	}
}
