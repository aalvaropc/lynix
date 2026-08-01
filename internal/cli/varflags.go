package cli

import (
	"fmt"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// parseVarFlags turns repeated --var key=value flags into a Vars map.
func parseVarFlags(flags []string) (domain.Vars, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	vars := domain.Vars{}
	for _, f := range flags {
		key, value, found := strings.Cut(f, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("invalid --var %q (expected key=value)", f)
		}
		// Builtins ($uuid, $env.X, ...) resolve before user vars, so an
		// override with a $-prefixed key would be silently ignored.
		if strings.HasPrefix(key, "$") {
			return nil, fmt.Errorf("invalid --var %q: keys starting with $ are reserved for builtins", f)
		}
		vars[key] = value
	}
	return vars, nil
}

// validateListFormat rejects unknown --format values on list commands so a
// typo fails loudly (exit 2) instead of silently printing the pretty output.
func validateListFormat(format string) error {
	switch format {
	case "", "pretty", "json":
		return nil
	default:
		return fmt.Errorf("unsupported format %q (expected pretty|json)", format)
	}
}
