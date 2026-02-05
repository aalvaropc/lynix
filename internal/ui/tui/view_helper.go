package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aalvaropc/lynix/internal/domain"
)

func clampString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))

	n := 0
	for _, r := range s {
		if n >= maxLen {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String() + "…"
}
func prettyBody(body []byte) string {
	if len(body) == 0 {
		return "(empty)"
	}
	var js any
	if err := json.Unmarshal(body, &js); err == nil {
		b, _ := json.MarshalIndent(js, "", "  ")
		return string(b)
	}
	return string(bytes.TrimSpace(body))
}

func renderResultDetails(rr domain.RequestResult) string {
	var b strings.Builder

	if rr.Error != nil {
		b.WriteString("Error:\n")
		b.WriteString("  - kind: ")
		b.WriteString(string(rr.Error.Kind))
		b.WriteString("\n  - msg: ")
		b.WriteString(rr.Error.Message)
		b.WriteString("\n\n")
	}

	b.WriteString(fmt.Sprintf("Status: %d\nLatency: %dms\n\n", rr.StatusCode, rr.LatencyMS))

	if len(rr.Assertions) > 0 {
		b.WriteString("Assertions:\n")
		for _, a := range rr.Assertions {
			status := "FAIL"
			if a.Passed {
				status = "PASS"
			}
			b.WriteString("  - ")
			b.WriteString(a.Name)
			b.WriteString(" [")
			b.WriteString(status)
			b.WriteString("] ")
			b.WriteString(a.Message)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rr.Extracts) > 0 {
		b.WriteString("Extracts:\n")
		for _, e := range rr.Extracts {
			status := "FAIL"
			if e.Success {
				status = "OK"
			}
			b.WriteString("  - ")
			b.WriteString(e.Name)
			b.WriteString(" [")
			b.WriteString(status)
			b.WriteString("] ")
			b.WriteString(e.Message)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rr.Extracted) > 0 {
		b.WriteString("Extracted Vars:\n")
		for k, v := range rr.Extracted {
			b.WriteString("  - ")
			b.WriteString(k)
			b.WriteString(" = ")
			b.WriteString(v)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderResultResponse(rr domain.RequestResult) string {
	var b strings.Builder

	b.WriteString("Headers:\n")
	if len(rr.Response.Headers) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for k, vals := range rr.Response.Headers {
			b.WriteString("  - ")
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(strings.Join(vals, ", "))
			b.WriteString("\n")
		}
	}

	b.WriteString("\nBody:\n")
	body := prettyBody(rr.Response.Body)
	if rr.Response.Truncated {
		body += "\n\n(truncated)"
	}
	b.WriteString(body)
	b.WriteString("\n")

	return b.String()
}
