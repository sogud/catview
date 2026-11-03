package main

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Read macOS preferred languages without cgo (MyGo builds with CGO_ENABLED=0).
// No shell is involved and no system preference is modified.
func systemLanguage() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return "en"
	}
	for _, field := range strings.Fields(string(data)) {
		tag := strings.Trim(field, "\"(),")
		if language := matchLanguage(tag); language != "" {
			return language
		}
	}
	return "en"
}
