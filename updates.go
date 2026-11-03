package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// The public release identifier includes the channel; Info.plist requires 0.1.0.
const releaseVersion = "0.1.0-alpha.2"
const releaseAPI = "https://api.github.com/repos/sogud/catview/releases?per_page=100"

type releaseInfo struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func (a *application) checkForUpdates() {
	if a.checkingUpdate {
		return
	}
	a.checkingUpdate = true
	win := a.win
	tr := a.translatorForUI()
	go func() {
		tag, err := latestRelease()
		latest := strings.TrimPrefix(tag, "v")
		options := mygo.MessageOptions{Parent: win, Message: tr("You’re up to date"), Detail: "CatView " + releaseVersion, Buttons: []string{tr("OK")}}
		if err != nil {
			options.Message = tr("Could not check for updates")
			options.Detail = tr(err.Error()) + "\n" + tr("Please try again later.")
		} else if newerVersion(latest, releaseVersion) {
			options.Message = fmt.Sprintf(tr("CatView %s is available"), latest)
			options.Detail = fmt.Sprintf(tr("Current version: %s. Download from the official release page; installation is manual."), releaseVersion)
			options.Buttons = []string{tr("Open download page"), tr("Later")}
			options.CancelButton = 1
		}
		result, dialogErr := mygo.Dialog.Message(options)
		if err == nil && dialogErr == nil && newerVersion(latest, releaseVersion) && result.Button == 0 {
			if openErr := mygo.Shell.OpenExternal("https://github.com/sogud/catview/releases/tag/" + url.PathEscape(tag)); openErr != nil {
				win.Update(func() { a.status = tr("Could not open download page: ") + openErr.Error() })
			}
		}
		win.Update(func() { a.checkingUpdate = false })
	}()
}

func latestRelease() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "CatView/"+releaseVersion)
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Could not connect to GitHub")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	var releases []releaseInfo
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&releases); err != nil {
		return "", fmt.Errorf("Could not read GitHub release information")
	}
	latest := ""
	latestTag := ""
	for _, release := range releases {
		version := strings.TrimPrefix(release.Tag, "v")
		if release.Draft || (release.Prerelease && !strings.Contains(releaseVersion, "-")) {
			continue
		}
		if _, _, ok := parseVersion(version); !ok {
			continue
		}
		if latest == "" || newerVersion(version, latest) {
			latest = version
			latestTag = release.Tag
		}
	}
	if latest == "" {
		return "", fmt.Errorf("No public release is available")
	}
	return latestTag, nil
}

func parseVersion(version string) ([3]uint64, []string, bool) {
	var numbers [3]uint64
	version = strings.SplitN(version, "+", 2)[0]
	parts := strings.SplitN(version, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return numbers, nil, false
	}
	for i, part := range core {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || (len(part) > 1 && part[0] == '0') {
			return numbers, nil, false
		}
		numbers[i] = value
	}
	var prerelease []string
	if len(parts) == 2 {
		prerelease = strings.Split(parts[1], ".")
		for _, part := range prerelease {
			if part == "" {
				return numbers, nil, false
			}
			if numericIdentifier(part) && len(part) > 1 && part[0] == '0' {
				return numbers, nil, false
			}
			for _, ch := range part {
				if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '-') {
					return numbers, nil, false
				}
			}
		}
	}
	return numbers, prerelease, true
}

func newerVersion(candidate, current string) bool {
	a, ap, aok := parseVersion(candidate)
	b, bp, bok := parseVersion(current)
	if !aok || !bok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	if len(ap) == 0 || len(bp) == 0 {
		return len(ap) == 0 && len(bp) != 0
	}
	for i := 0; i < min(len(ap), len(bp)); i++ {
		if ap[i] == bp[i] {
			continue
		}
		an, bn := numericIdentifier(ap[i]), numericIdentifier(bp[i])
		if an && bn && len(ap[i]) != len(bp[i]) {
			return len(ap[i]) > len(bp[i])
		}
		if an != bn {
			return !an
		}
		return ap[i] > bp[i]
	}
	return len(ap) > len(bp)
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
