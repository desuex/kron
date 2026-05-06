package daemon

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type logField struct {
	key   string
	value any
}

func formatLogEvent(ts time.Time, level, component, event string, fields ...logField) string {
	var b strings.Builder
	b.WriteString(ts.UTC().Format(time.RFC3339Nano))
	b.WriteString(" level=")
	b.WriteString(level)
	b.WriteString(" component=")
	b.WriteString(component)
	b.WriteString(" event=")
	b.WriteString(event)
	for _, field := range fields {
		if field.key == "" || field.value == nil {
			continue
		}
		text := fmt.Sprint(field.value)
		if text == "" {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(field.key)
		b.WriteByte('=')
		b.WriteString(formatLogValue(text))
	}
	b.WriteByte('\n')
	return b.String()
}

func writeLogEvent(w io.Writer, ts time.Time, level, component, event string, fields ...logField) {
	if w == nil {
		return
	}
	_, _ = io.WriteString(w, formatLogEvent(ts, level, component, event, fields...))
}

func formatLogValue(raw string) string {
	if raw == "" {
		return `""`
	}
	if strings.ContainsAny(raw, " \t\n\r\"\\") {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(raw)
		return `"` + escaped + `"`
	}
	return raw
}
