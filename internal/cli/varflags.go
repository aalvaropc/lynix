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
		vars[key] = value
	}
	return vars, nil
}
