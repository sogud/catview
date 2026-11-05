package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type decodedImage struct {
	picture  ui.ImageSource
	pages    int
	animated bool
}

func decodePreviewImage(data []byte) (ui.ImageSource, error) {
	decoded, err := decodeImagePage(data, 0)
	return decoded.picture, err
}

func (a *application) imageReadMenuItems() []*mygo.MenuItem {
	d := a.doc
	return []*mygo.MenuItem{
		{Label: fmt.Sprintf(a.tr("Image page %d of %d"), d.imageIndex+1, d.imagePages), Disabled: true},
		{Label: a.tr("Previous image page"), Accelerator: "Alt+Left", Disabled: d.loading || d.imageIndex == 0,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.changeImagePage(-1) }},
		{Label: a.tr("Next image page"), Accelerator: "Alt+Right", Disabled: d.loading || d.imageIndex+1 >= d.imagePages,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.changeImagePage(1) }},
	}
}

func (a *application) changeImagePage(delta int) {
	d := a.doc
	if d == nil || d.loading || d.imageIndex+delta < 0 || d.imageIndex+delta >= d.imagePages {
		return
	}
	d.imageIndex += delta
	d.imageScroll = ui.ScrollState{}
	a.loadDocument(d)
	a.installMenu()
	a.win.Invalidate()
}

// WebKit renders SVG in an image context: no document scripts, navigation or
// Go bridge. Reject active and external content as well as enforcing CSP.
func validateSVG(data []byte) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, nodes := 0, 0
	root := false
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("SVG 格式错误：%w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return errors.New("SVG 不支持 DTD 或外部实体")
		case xml.StartElement:
			nodes++
			depth++
			if nodes > 20000 || depth > 80 {
				return errors.New("SVG 超过预览复杂度限制")
			}
			if !root {
				if t.Name.Local != "svg" {
					return errors.New("文件不是 SVG 图像")
				}
				root = true
			}
			switch strings.ToLower(t.Name.Local) {
			case "script", "foreignobject", "iframe", "object", "embed", "image", "audio", "video", "animate", "animatetransform", "animatemotion", "set":
				return errors.New("SVG 包含活动内容或嵌入资源，暂不预览")
			}
			for _, attr := range t.Attr {
				name, value := strings.ToLower(attr.Name.Local), strings.TrimSpace(attr.Value)
				if strings.HasPrefix(name, "on") || ((name == "href" || name == "src") && !strings.HasPrefix(value, "#")) {
					return errors.New("SVG 包含脚本或外部链接，暂不预览")
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if !root {
		return errors.New("SVG 内容为空")
	}
	return nil
}

func (a *application) imageHTML(d *document) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(d.source))
	mime := "image/svg+xml"
	if d.imageAnimated {
		mime = "image/gif"
	}
	// The only script is ours. An SVG in an <img> cannot execute script or
	// fetch external resources; CSP adds a second restriction on this page.
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src 'nonce-catview-image'; base-uri 'none'; form-action 'none'">
<style>:root{color-scheme:light dark}html,body{margin:0;width:100%%;height:100%%;background:%s}body{overflow:auto}main{min-width:100%%;min-height:100%%;display:grid;place-items:center}img{display:block}body.fit main{width:100%%;height:100%%}body.fit img{max-width:calc(100vw - 44px);max-height:calc(100vh - 44px);width:auto;height:auto}</style></head>
<body class="fit"><main><img alt="" src="data:%s;base64,%s"></main>
<script nonce="catview-image">const picture=document.querySelector('img');let scale=%g;window.catviewImageZoom=(s)=>{scale=s;document.body.classList.toggle('fit',!s);picture.style.width=s?picture.naturalWidth*s+'px':'';picture.style.height=s?picture.naturalHeight*s+'px':''};picture.onload=()=>window.catviewImageZoom(scale);</script></body></html>`, a.webBackground(), mime, encoded, d.imageZoom)
}
