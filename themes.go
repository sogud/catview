package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type surfaceColors struct {
	paper, surface, ink, muted, border string
}

type colorTheme struct {
	id, name    string
	accent      int
	light, dark surfaceColors
}

// Paired presets keep reading surfaces quiet in both appearances. Accent
// colors remain a separate choice and never tint the whole document.
var colorThemes = []colorTheme{
	{"macos", "macOS", 1, surfaceColors{"#FFFFFF", "#F0F0F0", "#262626", "#6C6C70", "#DEDEDE"}, surfaceColors{"#1E1E1E", "#292929", "#F2F2F2", "#ABABAF", "#404040"}},
	{"paper", "Paper", 0, surfaceColors{"#FBFAF7", "#F1F2F1", "#2B3034", "#69747B", "#E0E4E1"}, surfaceColors{"#202225", "#2A2C2F", "#EDF0F1", "#ABB3B9", "#3C4144"}},
	{"github", "GitHub", 1, surfaceColors{"#FFFFFF", "#F6F8FA", "#1F2328", "#59636E", "#D1D9E0"}, surfaceColors{"#0D1117", "#161B22", "#E6EDF3", "#9DA7B3", "#30363D"}},
	{"nord", "Nord", 5, surfaceColors{"#F5F7FA", "#ECEFF4", "#2E3440", "#596579", "#D8DEE9"}, surfaceColors{"#2E3440", "#363E4C", "#ECEFF4", "#BBC5D5", "#4C566A"}},
	{"forest", "Forest", 2, surfaceColors{"#F8F9F5", "#EFF2EB", "#2C3830", "#5F7063", "#DCE3D8"}, surfaceColors{"#232B28", "#2C3531", "#E4ECE4", "#ADBCB0", "#435048"}},
	{"lavender", "Lavender", 3, surfaceColors{"#FAF9FC", "#F0EEF5", "#343242", "#6B667D", "#E0DCE9"}, surfaceColors{"#24232D", "#2E2C39", "#EAE7F2", "#B7B0CA", "#464255"}},
	{"sand", "Sand", 4, surfaceColors{"#FCFAF5", "#F3F0E8", "#38352F", "#736B5B", "#E3DDD1"}, surfaceColors{"#282724", "#32302B", "#ECE7DC", "#BDB4A3", "#4B463D"}},
}

func (a *application) sidebarMaterial() mygo.Vibrancy {
	if a.colorTheme == "macos" {
		return mygo.VibrancySidebar
	}
	return mygo.VibrancyNone
}

func (a *application) applySystemAppearance() {
	source := mygo.ThemeSystem
	if a.themeMode == 1 {
		source = mygo.ThemeLight
	}
	if a.themeMode == 2 {
		source = mygo.ThemeDark
	}
	mygo.Theme.SetSource(source)
}

func (a *application) selectedTheme() colorTheme {
	for _, theme := range colorThemes {
		if theme.id == a.colorTheme {
			return theme
		}
	}
	return colorThemes[0]
}

func (a *application) colors(dark bool) surfaceColors {
	theme := a.selectedTheme()
	if dark {
		return theme.dark
	}
	return theme.light
}

func (a *application) themeCSSVariables() string {
	var css strings.Builder
	for _, appearance := range []struct {
		name   string
		colors surfaceColors
	}{{"light", a.colors(false)}, {"dark", a.colors(true)}} {
		p := appearance.colors
		for _, pair := range [][2]string{{"paper", p.paper}, {"panel", p.surface}, {"ink", p.ink}, {"muted", p.muted}, {"line", p.border}} {
			css.WriteString("--" + pair[0] + "-" + appearance.name + ":" + pair[1] + ";")
		}
	}
	return css.String()
}

func (a *application) webBackground() string {
	if a.themeMode == 1 {
		return a.colors(false).paper
	}
	if a.themeMode == 2 {
		return a.colors(true).paper
	}
	return "light-dark(" + a.colors(false).paper + "," + a.colors(true).paper + ")"
}

// Window chrome stays neutral even when the document uses a colored theme.
func windowChromeTheme(dark bool) *ui.Theme {
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.Focus = t.TextMuted.Alpha(0.5)
	return &t
}
