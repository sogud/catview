package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

func (a *application) fontSettings(c *ui.Context) {
	families := ui.FontFamilies()
	ui.Form(c, func() {
		for _, option := range []struct {
			label string
			value *string
			mono  bool
		}{
			{"Reading font", &a.readingFont, false}, {"Code font", &a.codeFont, true},
		} {
			ui.Field(c, a.tr(option.label), func() {
				names := []string{a.tr("System")}
				found := *option.value == ""
				for _, family := range families {
					if option.mono && !family.Monospaced {
						continue
					}
					names = append(names, family.Name)
					if family.Name == *option.value {
						found = true
					}
				}
				if !found {
					names = append(names, *option.value)
				}
				selected := *option.value
				if selected == "" {
					selected = a.tr("System")
				}
				if ui.Select(c, &selected, names).Width(240).AlignSelf(ui.Start).Label(a.tr(option.label)).Changed() {
					if selected == a.tr("System") {
						selected = ""
					}
					*option.value = selected
					a.settingsChanged()
				}
			})
		}
	})
	codeFont := a.codeFont
	if codeFont == "" {
		codeFont = "monospace"
	}
	ui.Text(c, "Aa 中文 0123 · func main() {}").Font(codeFont).FontSize(a.fontSize)
}

func cssFontFamily(name, fallback string) string {
	if name == "" {
		return fallback
	}
	name = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "<", "\\3c ", ">", "\\3e ", "\n", " ", "\r", " ").Replace(name)
	return "\"" + name + "\"," + fallback
}

func (a *application) fontCSS() string {
	css := ""
	if a.readingFont != "" {
		css += "body,#chapter{font-family:" + cssFontFamily(a.readingFont, "sans-serif") + "}"
	}
	if a.codeFont != "" {
		css += "pre,code,kbd,#chapter pre,#chapter code{font-family:" + cssFontFamily(a.codeFont, "ui-monospace,monospace") + "}"
	}
	return css
}
