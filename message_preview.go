package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
)

func textParagraph(value string) string {
	return strings.ReplaceAll(html.EscapeString(value), "\n", "<br>")
}

// go-message owns MIME, transfer encoding and charset decoding. CatView only
// chooses the body and lists attachments; no attachment is opened or saved.
func readMessagePreview(ctx context.Context, data []byte) (string, error) {
	root, err := message.ReadWithOptions(&previewReader{ctx: ctx, reader: bytes.NewReader(data)}, &message.ReadOptions{MaxHeaderBytes: 64 << 10})
	if err != nil {
		return "", fmt.Errorf("无法解析邮件：%w", err)
	}
	var plain, rich string
	var attachments []string
	parts, bodyBytes := 0, 0
	var visit func(*message.Entity, int) error
	visit = func(entity *message.Entity, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		parts++
		if depth > 16 || parts > 200 {
			return errors.New("邮件超过 16 层或 200 个 MIME 部件的预览限制")
		}
		if reader := entity.MultipartReader(); reader != nil {
			defer reader.Close()
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				if err := visit(part, depth+1); err != nil {
					return err
				}
			}
		}
		mediaType, params, err := entity.Header.ContentType()
		if err != nil {
			return err
		}
		disposition, dispositionParams, err := entity.Header.ContentDisposition()
		if err != nil && entity.Header.Get("Content-Disposition") != "" {
			return err
		}
		if disposition == "attachment" || (mediaType != "text/plain" && mediaType != "text/html") {
			name := dispositionParams["filename"]
			if name == "" {
				name = params["name"]
			}
			if name == "" {
				name = mediaType
			}
			attachments = append(attachments, name)
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(&previewReader{ctx: ctx, reader: entity.Body}, 2<<20+1))
		if err != nil {
			return err
		}
		bodyBytes += len(body)
		if len(body) > 2<<20 || bodyBytes > 4<<20 {
			return errors.New("邮件正文超过预览限制（单段 2 MiB、合计 4 MiB）")
		}
		if mediaType == "text/html" && rich == "" {
			rich = string(body)
		} else if mediaType == "text/plain" && plain == "" {
			plain = string(body)
		}
		return nil
	}
	if err := visit(root, 0); err != nil {
		return "", fmt.Errorf("无法解析邮件内容：%w", err)
	}
	subject, _ := root.Header.Text("Subject")
	if subject == "" {
		subject = "无主题"
	}
	var out strings.Builder
	out.WriteString("<h1>" + html.EscapeString(subject) + "</h1><dl>")
	for _, field := range []struct{ key, label string }{{"From", "发件人"}, {"To", "收件人"}, {"Cc", "抄送"}, {"Date", "日期"}} {
		value, _ := root.Header.Text(field.key)
		if value != "" {
			out.WriteString("<dt>" + field.label + "</dt><dd>" + html.EscapeString(value) + "</dd>")
		}
	}
	out.WriteString("</dl><hr>")
	if rich != "" {
		out.WriteString(rich) // Sanitized later with local/remote images disabled.
	} else {
		out.WriteString("<p>" + textParagraph(plain) + "</p>")
	}
	if len(attachments) > 0 {
		out.WriteString("<hr><h2>附件</h2><ul>")
		for _, name := range attachments {
			out.WriteString("<li>" + html.EscapeString(name) + "</li>")
		}
		out.WriteString("</ul>")
	}
	out.WriteString("<p><small>只读邮件预览；图片不加载，附件不打开。</small></p>")
	return out.String(), nil
}
