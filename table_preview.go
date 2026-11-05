package main

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	maxTableRows    = 20000
	maxTableColumns = 128
	maxTableCells   = 200000
)

type tablePreview struct {
	columns   []ui.TableColumn
	rows      [][]string
	notice    string
	state     ui.ListState
	selection ui.Selection[int]
}

func readDelimited(ctx context.Context, source string, tab bool) (*tablePreview, error) {
	r := csv.NewReader(strings.NewReader(source))
	if tab {
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1
	t := &tablePreview{}
	width, cells := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("表格格式错误：%w", err)
		}
		if len(row) > maxTableColumns {
			row = row[:maxTableColumns]
			t.notice = "Preview limited to 20,000 rows, 128 columns or 200,000 cells."
		}
		if len(t.rows) == maxTableRows || cells+len(row) > maxTableCells {
			t.notice = "Preview limited to 20,000 rows, 128 columns or 200,000 cells."
			break
		}
		cells += len(row)
		width = max(width, len(row))
		t.rows = append(t.rows, row)
	}
	t.columns = []ui.TableColumn{{Title: "", ID: "row", Width: 56, RowHeader: true, Fixed: true, Align: ui.End}}
	for i := 0; i < width; i++ {
		name := columnName(i)
		t.columns = append(t.columns, ui.TableColumn{Title: name, ID: name, Width: 160, MinWidth: 64, MaxWidth: 800})
	}
	return t, nil
}

func columnName(n int) string {
	name := ""
	for n++; n > 0; n = (n - 1) / 26 {
		name = string(rune('A'+(n-1)%26)) + name
	}
	return name
}

func (t *tablePreview) cell(row, column int) string {
	if row < 0 || row >= len(t.rows) || column < 0 {
		return ""
	}
	if column == 0 {
		return fmt.Sprint(row + 1)
	}
	if column > len(t.rows[row]) {
		return ""
	}
	return t.rows[row][column-1]
}

func (a *application) tableView(c *ui.Context, d *document) {
	t := d.table
	if t == nil {
		return
	}
	if d.showSource && d.code != nil {
		a.codeView(c, d)
		return
	}
	if t.notice != "" {
		ui.Text(c, a.tr(t.notice)).Padding(10, 16).TextColor(c.Theme().TextMuted)
	}
	if len(t.rows) == 0 {
		a.infoView(c, a.tr("Empty table"), "")
		return
	}
	t.state.Selection = &t.selection
	t.state.CellSelect, t.state.DragSelect = true, true
	t.state.CellMenu = func(m *ui.Menu, row, col int) {
		if m.Item(a.tr("Copy cell")).Chosen() {
			mygo.Clipboard.WriteText(t.cell(row, col))
		}
	}
	ui.Table(c, &t.state, t.columns, len(t.rows), func(row, col int) {
		value := t.cell(row, col)
		// Multiline cells remain one row tall; their complete value is in the
		// tooltip and copy action, so a quoted newline never hides later rows.
		ui.Text(c, strings.ReplaceAll(value, "\n", " ↵ ")).SingleLine().Tooltip(value).Padding(5, 8)
	}).Grow(1).FillWidth()
}

func readZIPCatalog(ctx context.Context, path string) (*tablePreview, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	if len(z.File) > maxTableRows {
		return nil, errors.New("ZIP 条目超过 20,000 个，暂不加载")
	}
	t := &tablePreview{notice: "Archive contents only. Files are not extracted."}
	t.columns = []ui.TableColumn{
		{Title: "", ID: "row", Width: 56, RowHeader: true, Fixed: true},
		{Title: "Name", ID: "name", Width: 420, MinWidth: 120},
		{Title: "Size", ID: "size", Width: 120, Align: ui.End},
		{Title: "Compressed", ID: "compressed", Width: 120, Align: ui.End},
	}
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Listing only: never join archive paths to filesystem paths, extract,
		// follow symlinks, or allocate an entry's declared uncompressed size.
		t.rows = append(t.rows, []string{f.Name, fmt.Sprintf("%d B", f.UncompressedSize64), fmt.Sprintf("%d B", f.CompressedSize64)})
	}
	return t, nil
}
