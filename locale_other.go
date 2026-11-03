//go:build !darwin

package main

import "os"

func systemLanguage() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if language := matchLanguage(os.Getenv(key)); language != "" {
			return language
		}
	}
	return "en"
}
