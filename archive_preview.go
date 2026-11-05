package main

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func isCatalogArchive(path string) bool {
	name := strings.ToLower(path)
	for _, suffix := range []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".7z", ".rar", ".cbr"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// TAR uses the standard library. 7z/RAR use the system libarchive tool;
// neither path extracts entries or follows archive links.
func readArchiveCatalog(ctx context.Context, path string) (*tablePreview, error) {
	f, err := openPreviewFile(path, 100<<20)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	t := &tablePreview{notice: "Archive contents only. Files are not extracted.", columns: []ui.TableColumn{
		{Title: "", ID: "row", Width: 56, RowHeader: true, Fixed: true},
		{Title: "Name", ID: "name", Width: 420, MinWidth: 120},
	}}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".7z" || ext == ".rar" || ext == ".cbr" {
		if runtime.GOOS != "darwin" {
			return nil, errors.New("此格式的目录预览需要 macOS 系统 libarchive")
		}
		header := make([]byte, 8)
		if _, err := io.ReadFull(f, header); err != nil {
			return nil, err
		}
		valid := bytes.HasPrefix(header, []byte("7z\xbc\xaf\x27\x1c")) && ext == ".7z"
		if ext != ".7z" {
			valid = bytes.HasPrefix(header, []byte("Rar!\x1a\x07\x00")) || bytes.Equal(header, []byte("Rar!\x1a\x07\x01\x00"))
		}
		if !valid {
			return nil, errors.New("压缩包签名与扩展名不符")
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		// Pass the already checked descriptor, not a path that can be replaced.
		cmd := exec.CommandContext(ctx, "/usr/bin/bsdtar", "-t", "-f", "/dev/fd/3")
		cmd.ExtraFiles = append(cmd.ExtraFiles, f)
		output := &boundedPreviewOutput{limit: 2 << 20, cancel: cancel}
		stderr := &boundedPreviewOutput{limit: 64 << 10, cancel: cancel}
		cmd.Stdout, cmd.Stderr = output, stderr
		cmd.WaitDelay = time.Second
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("系统无法读取压缩包目录（不支持的压缩方式、加密、损坏或超出 8 秒/输出限制）：%w", err)
		}
		for _, name := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
			if name != "" {
				t.rows = append(t.rows, []string{name})
			}
			if len(t.rows) > maxTableRows {
				return nil, errors.New("压缩包条目超过 20,000 个")
			}
		}
		return t, nil
	}
	t.columns = append(t.columns, ui.TableColumn{Title: "Size", ID: "size", Width: 120, Align: ui.End})
	var source io.Reader = &previewReader{ctx: ctx, reader: f}
	if ext == ".gz" || ext == ".tgz" {
		compressed, err := gzip.NewReader(source)
		if err != nil {
			return nil, err
		}
		defer compressed.Close()
		source = compressed
	} else if ext == ".bz2" || ext == ".tbz2" {
		source = bzip2.NewReader(source)
	}
	stream := &budgetPreviewReader{reader: &previewReader{ctx: ctx, reader: source}, remaining: 256 << 20}
	archive := tar.NewReader(stream)
	for {
		entry, err := archive.Next()
		if err == io.EOF {
			return t, nil
		}
		if err != nil {
			return nil, fmt.Errorf("无法读取 TAR 目录：%w", err)
		}
		if len(t.rows) >= maxTableRows {
			return nil, errors.New("TAR 条目超过 20,000 个")
		}
		name := entry.Name
		if entry.Linkname != "" {
			name += " → " + entry.Linkname
		}
		t.rows = append(t.rows, []string{name, fmt.Sprintf("%d B", entry.Size)})
	}
}

// Return a real error on budget exhaustion; EOF would disguise truncation.
type budgetPreviewReader struct {
	reader    io.Reader
	remaining int64
}

func (r *budgetPreviewReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, errors.New("TAR 扫描超过 256 MiB 解压数据限制")
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

type boundedPreviewOutput struct {
	bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (w *boundedPreviewOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.Len() {
		w.cancel()
		return 0, errors.New("系统预览输出超过限制")
	}
	return w.Buffer.Write(p)
}
