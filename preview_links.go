package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// Keep in-document anchors native to the web page. Local file links become
// app navigation; the web page never receives access to the local filesystem.
func previewLink(sourceFile, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.ContainsAny(ref, "\x00\\") || strings.HasPrefix(ref, "//") {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "mailto" || strings.HasPrefix(ref, "#") {
		return ref
	}
	if u.Scheme != "" && u.Scheme != "file" || u.Host != "" && !(u.Scheme == "file" && u.Host == "localhost") {
		return ""
	}
	path := u.Path // url.Parse already decodes percent-encoded path bytes once.
	if path == "" {
		path = sourceFile
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(sourceFile), filepath.FromSlash(path))
	}
	q := url.Values{"path": {filepath.Clean(path)}, "fragment": {u.Fragment}}
	return "catview://open?" + q.Encode()
}

func (a *application) openPreviewLink(path, fragment, scope string) {
	if a.fileOperation {
		return
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && !pathInScope(scope, resolved) {
		err = fmt.Errorf("%s", a.tr("The linked file is outside the opened folder."))
	}
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(resolved)
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("%s", a.tr("This link does not point to a file."))
		}
	}
	if err != nil {
		message := err.Error()
		if os.IsNotExist(err) {
			message = a.tr("The linked file could not be found.")
		}
		go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageWarning,
			Message: a.tr("Cannot open link"), Detail: filepath.Base(path) + "\n" + message,
			Buttons: []string{a.tr("OK")}})
		return
	}
	a.openDocumentAt(resolved, scope, fragment)
}
