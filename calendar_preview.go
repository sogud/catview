package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
)

func calendarTime(prop *ical.Prop) string {
	if prop == nil {
		return ""
	}
	value, err := prop.DateTime(time.UTC)
	zone := prop.Params.Get("TZID")
	if err != nil {
		return prop.Value + "（原始时间，时区：" + zone + "）"
	}
	if prop.ValueType() == ical.ValueDate || len(prop.Value) == 8 {
		return value.Format("2006-01-02") + "（全天）"
	}
	if zone == "" {
		zone = "未指定时区"
		if strings.HasSuffix(prop.Value, "Z") {
			zone = "UTC"
		}
	}
	return value.Format("2006-01-02 15:04:05") + " · " + zone
}

// Input bounds and panic isolation protect the app around the external
// parser. Recurrence rules are displayed, never expanded into occurrences.
func readCalendarPreview(ctx context.Context, data []byte) (output string, err error) {
	defer func() {
		if recover() != nil {
			output, err = "", errors.New("日历结构无效，无法预览")
		}
	}()
	lines := bufio.NewScanner(bytes.NewReader(data))
	lines.Buffer(make([]byte, 4096), 8192)
	logicalLine := ""
	components, depth, lineCount := 0, 0, 0
	checkLine := func() error {
		line := strings.ToUpper(logicalLine)
		if strings.HasPrefix(line, "BEGIN:") {
			components++
			depth++
		} else if strings.HasPrefix(line, "END:") {
			depth--
		}
		if len(logicalLine) > 4096 || components > 512 || depth > 32 || lineCount > 20000 {
			return errors.New("日历超过预览复杂度限制（逻辑行 4 KiB、512 个组件、32 层）")
		}
		return nil
	}
	for lines.Scan() {
		line := lines.Text()
		lineCount++
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			logicalLine += line[1:]
		} else {
			if err := checkLine(); err != nil {
				return "", err
			}
			logicalLine = line
		}
		if len(logicalLine) > 4096 {
			return "", errors.New("日历逻辑行超过 4 KiB")
		}
	}
	if err := checkLine(); err != nil {
		return "", err
	}
	if err := lines.Err(); err != nil {
		return "", fmt.Errorf("日历行长度超过限制：%w", err)
	}
	calendar, err := ical.NewDecoder(&previewReader{ctx: ctx, reader: bytes.NewReader(data)}).Decode()
	if err != nil {
		return "", fmt.Errorf("无法解析日历：%w", err)
	}
	var out strings.Builder
	out.WriteString("<h1>日历</h1>")
	events := calendar.Events()
	if len(events) == 0 {
		out.WriteString("<p>没有可显示的 VEVENT 事件；可从显示菜单查看源码。</p>")
	}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		title, _ := event.Props.Text(ical.PropSummary)
		if title == "" {
			title = "无标题事件"
		}
		out.WriteString("<h2>" + html.EscapeString(title) + "</h2><dl>")
		fields := []struct{ label, value string }{
			{"开始", calendarTime(event.Props.Get(ical.PropDateTimeStart))},
			{"结束（不含）", calendarTime(event.Props.Get(ical.PropDateTimeEnd))},
		}
		for _, key := range []struct{ key, label string }{{"LOCATION", "地点"}, {"STATUS", "状态"}, {"DURATION", "时长"}, {"RRULE", "重复规则（未展开）"}, {"ORGANIZER", "组织者"}} {
			if prop := event.Props.Get(key.key); prop != nil {
				value := prop.Value
				if text, e := prop.Text(); e == nil {
					value = text
				}
				fields = append(fields, struct{ label, value string }{key.label, value})
			}
		}
		for _, field := range fields {
			if field.value != "" {
				out.WriteString("<dt>" + field.label + "</dt><dd>" + textParagraph(field.value) + "</dd>")
			}
		}
		out.WriteString("</dl>")
		if description, _ := event.Props.Text(ical.PropDescription); description != "" {
			out.WriteString("<p>" + textParagraph(description) + "</p>")
		}
		out.WriteString("<hr>")
	}
	out.WriteString("<p><small>仅查看事件；不导入日历、不执行提醒。自定义时区无法解析时保留原值。</small></p>")
	return out.String(), nil
}
