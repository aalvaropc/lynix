package template

import (
	"fmt"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// RenderString replaces {{VAR}} placeholders with vars values.
// It returns an error if a variable is missing or a placeholder is malformed.
func RenderString(input string, vars map[string]string) (string, error) {
	if input == "" {
		return "", nil
	}

	var out strings.Builder
	rest := input
	for {
		start := strings.Index(rest, "{{")
		if start == -1 {
			out.WriteString(rest)
			return out.String(), nil
		}

		out.WriteString(rest[:start])
		rest = rest[start+2:]

		end := strings.Index(rest, "}}")
		if end == -1 {
			return "", &domain.Error{
				Kind: domain.KindInvalidConfig,
				Msg:  "unclosed template expression",
			}
		}

		key := strings.TrimSpace(rest[:end])
		if key == "" {
			return "", &domain.Error{
				Kind: domain.KindInvalidConfig,
				Msg:  "empty template expression",
			}
		}

		value, ok := vars[key]
		if !ok {
			return "", &domain.Error{
				Kind: domain.KindMissingVar,
				Msg:  fmt.Sprintf("missing variable %q", key),
			}
		}

		out.WriteString(value)
		rest = rest[end+2:]
	}
}
