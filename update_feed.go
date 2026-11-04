package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/netproxy"
)

const releasePage = "https://github.com/sogud/catview/releases"
const releaseAPI = "https://api.github.com/repos/sogud/catview/releases?per_page=100"
const maxUpdateSize = 256 << 20

//go:embed mygo-update.pub
var updatePublicKey string

var updateClient = &http.Client{
	Transport: netproxy.Transport(),
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || req.URL.Scheme != "https" {
			return errors.New("Invalid update redirect")
		}
		switch req.URL.Host {
		case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
		return errors.New("Invalid update download host")
	},
}

type updateHTTPError struct {
	status     int
	retryAfter time.Time
}

func (e *updateHTTPError) Error() string { return fmt.Sprintf("GitHub returned HTTP %d", e.status) }

func updateGET(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CatView/"+releaseVersion)
	response, err := updateClient.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		e := &updateHTTPError{status: response.StatusCode}
		if response.Header.Get("X-RateLimit-Remaining") == "0" {
			if reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				e.retryAfter = time.Unix(reset, 0)
			}
		}
		if retry := response.Header.Get("Retry-After"); retry != "" {
			if seconds, err := strconv.Atoi(retry); err == nil {
				e.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
			} else if date, err := http.ParseTime(retry); err == nil {
				e.retryAfter = date
			}
		}
		return nil, e
	}
	return response.Body, nil
}

func latestRelease(ctx context.Context) (string, error) {
	// The public Atom feed includes prereleases without consuming REST quota.
	if tag, err := latestAtomRelease(ctx); err == nil {
		return tag, nil
	}
	body, err := updateGET(ctx, releaseAPI)
	if err != nil {
		return "", err
	}
	defer body.Close()
	var releases []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 4<<20)).Decode(&releases); err != nil {
		return "", errors.New("Could not read GitHub release information")
	}
	best := ""
	for _, r := range releases {
		if !r.Draft && (!r.Prerelease || strings.Contains(releaseVersion, "-")) {
			best = newerReleaseTag(best, r.Tag)
		}
	}
	if best == "" {
		return "", errors.New("No public release is available")
	}
	return best, nil
}

func latestAtomRelease(ctx context.Context) (string, error) {
	body, err := updateGET(ctx, releasePage+".atom")
	if err != nil {
		return "", err
	}
	defer body.Close()
	var feed struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		Entries []struct {
			Links []struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.NewDecoder(io.LimitReader(body, 4<<20)).Decode(&feed); err != nil {
		return "", err
	}
	best := ""
	for _, entry := range feed.Entries {
		for _, link := range entry.Links {
			if tag, ok := strings.CutPrefix(link.Href, releasePage+"/tag/"); ok {
				best = newerReleaseTag(best, tag)
			}
		}
	}
	if best == "" {
		return "", errors.New("No public release is available")
	}
	return best, nil
}

func newerReleaseTag(best, tag string) string {
	version, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return best
	}
	if _, _, ok := parseVersion(version); !ok || (!strings.Contains(releaseVersion, "-") && strings.Contains(version, "-")) {
		return best
	}
	if best == "" || newerVersion(version, strings.TrimPrefix(best, "v")) {
		return tag
	}
	return best
}

type updateManifest struct {
	Version   string `json:"version"`
	Notes     string `json:"notes,omitempty"`
	Date      string `json:"date,omitempty"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

// preparedUpdate serves only the verified archive in our cache to MyGo.
// Downloading never invokes MyGo.Install, which would stage a replacement.
type preparedUpdate struct {
	manifest updateManifest
	data     []byte
	path     string
}

func prepareUpdate(data []byte, cacheDir string) (*preparedUpdate, error) {
	var m updateManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errors.New("Invalid update manifest")
	}
	if _, _, ok := parseVersion(m.Version); !ok || m.Size <= 0 || m.Size > maxUpdateSize {
		return nil, errors.New("Invalid update manifest")
	}
	name := "catview-" + m.Version + "-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	expected := releasePage + "/download/v" + url.PathEscape(m.Version) + "/" + name
	if m.URL != expected {
		return nil, errors.New("Invalid update download URL")
	}
	if sig, err := base64.StdEncoding.DecodeString(m.Signature); err != nil || len(sig) != ed25519.SignatureSize {
		return nil, errors.New("Invalid update signature")
	}
	// Omit server-provided deltas: auto-download and installation use exactly
	// the same signed archive, including when the user restarts offline.
	clean, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &preparedUpdate{manifest: m, data: clean, path: filepath.Join(cacheDir, name)}, nil
}

func findUpdate(ctx context.Context, cacheDir string) (*preparedUpdate, error) {
	if cacheDir == "" {
		return nil, errors.New("Could not create the update cache")
	}
	pending := filepath.Join(cacheDir, "pending.json")
	if data, err := os.ReadFile(pending); err == nil {
		if source, err := prepareUpdate(data, cacheDir); err == nil {
			if newerVersion(source.manifest.Version, releaseVersion) && source.cached() == nil {
				return source, nil
			}
			if !newerVersion(source.manifest.Version, releaseVersion) {
				_ = os.Remove(source.path)
			}
		}
		_ = os.Remove(pending)
	}
	tag, err := latestRelease(ctx)
	if err != nil || !newerVersion(strings.TrimPrefix(tag, "v"), releaseVersion) {
		return nil, err
	}
	manifestURL := releasePage + "/download/" + url.PathEscape(tag) + "/update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	body, err := updateGET(ctx, manifestURL)
	if err != nil {
		var status *updateHTTPError
		if errors.As(err, &status) && status.status == http.StatusNotFound {
			return nil, errors.New("This release requires a manual download. Open the download page to install it.")
		}
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, err
	}
	source, err := prepareUpdate(data, cacheDir)
	if err != nil {
		return nil, err
	}
	if "v"+source.manifest.Version != tag {
		return nil, errors.New("Update version does not match the release")
	}
	return source, nil
}

func (s *preparedUpdate) Manifest(context.Context, bool) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s *preparedUpdate) Open(_ context.Context, rawURL string) (io.ReadCloser, error) {
	if rawURL != s.manifest.URL {
		return nil, errors.New("Invalid update download URL")
	}
	return os.Open(s.path)
}

func (s *preparedUpdate) verifyDigest(digest []byte) error {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(updatePublicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("Invalid update public key")
	}
	signature, err := base64.StdEncoding.DecodeString(s.manifest.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(key), digest, signature) {
		return errors.New("Update signature verification failed. The app was not changed.")
	}
	return nil
}

func (s *preparedUpdate) cached() error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	n, err := io.Copy(sum, io.LimitReader(f, s.manifest.Size+1))
	if err != nil {
		return err
	}
	if n != s.manifest.Size {
		return errors.New("Incomplete update download")
	}
	if err := s.verifyDigest(sum.Sum(nil)); err != nil {
		return err
	}
	return s.verifyBundle(s.path)
}

type updateProgressWriter struct {
	n        int64
	progress func(int64)
}

func (w *updateProgressWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	w.progress(w.n)
	return len(p), nil
}

func (s *preparedUpdate) download(ctx context.Context, progress func(int64)) error {
	if s.cached() == nil {
		return writePreferences(filepath.Join(filepath.Dir(s.path), "pending.json"), s.data)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	body, err := updateGET(ctx, s.manifest.URL)
	if err != nil {
		return err
	}
	defer body.Close()
	f, err := os.CreateTemp(filepath.Dir(s.path), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, sum, &updateProgressWriter{progress: progress}), io.LimitReader(body, s.manifest.Size+1))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n != s.manifest.Size {
		return errors.New("Incomplete update download")
	}
	if err := s.verifyDigest(sum.Sum(nil)); err != nil {
		return err
	}
	if err := s.verifyBundle(f.Name()); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	return writePreferences(filepath.Join(filepath.Dir(s.path), "pending.json"), s.data)
}

func (s *preparedUpdate) verifyBundle(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(io.LimitReader(gz, 1<<30))
	for {
		h, err := reader.Next()
		if err != nil {
			return errors.New("Invalid CatView update bundle")
		}
		if h.Name != "CatView.app/Contents/Info.plist" || h.Typeflag != tar.TypeReg {
			continue
		}
		if h.Size > 1<<20 {
			return errors.New("Invalid CatView update bundle")
		}
		var plist struct {
			Dict struct {
				Entries []struct {
					XMLName xml.Name
					Text    string `xml:",chardata"`
				} `xml:",any"`
			} `xml:"dict"`
		}
		if err := xml.NewDecoder(reader).Decode(&plist); err != nil {
			return err
		}
		values := map[string]string{}
		for i := 0; i+1 < len(plist.Dict.Entries); i++ {
			key, value := plist.Dict.Entries[i], plist.Dict.Entries[i+1]
			if key.XMLName.Local == "key" && value.XMLName.Local == "string" {
				values[key.Text] = value.Text
			}
		}
		// The signed plist binds the archive to its advertised app/version,
		// so an unsigned manifest cannot relabel an older signed archive.
		if values["CFBundleIdentifier"] != "me.sogud.catview" || values["CatViewReleaseVersion"] != s.manifest.Version {
			return errors.New("Update version does not match the release")
		}
		return nil
	}
}
