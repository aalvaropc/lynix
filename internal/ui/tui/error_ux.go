package tui

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

var reLine = regexp.MustCompile(`(?i)\bline\s+(\d+)\b`)

func userMessage(err error) string {
	if err == nil {
		return ""
	}

	var oe *domain.OpError
	if errors.As(err, &oe) {
		switch oe.Kind {

		case domain.KindNotFound:
			if strings.Contains(oe.Op, "yamlcollection") {
				return "Collection not found"
			}
			if strings.Contains(oe.Op, "yamlenv") {
				return "Environment not found"
			}
			if strings.Contains(oe.Op, "workspacefinder.findroot") {
				return "Workspace not found"
			}
			return "Not found"

		case domain.KindMissingVar:
			v := extractMissingVarName(err.Error())
			if v == "" {
				return "Missing variable"
			}
			return "Missing variable " + v

		case domain.KindInvalidConfig:
			base := "config"
			if strings.TrimSpace(oe.Path) != "" {
				base = filepath.Base(oe.Path)
			}

			line := extractLine(err.Error())
			if line != "" {
				return "Invalid YAML at " + base + " line " + line
			}

			if looksLikeYAMLProblem(err.Error()) {
				return "Invalid YAML at " + base
			}
			return "Invalid config"

		default:
			return "Unexpected error (see logs)"
		}
	}

	if looksLikeYAMLProblem(err.Error()) {
		line := extractLine(err.Error())
		if line != "" {
			return "Invalid YAML line " + line
		}
		return "Invalid YAML"
	}
	if strings.Contains(strings.ToLower(err.Error()), "missing variable") {
		v := extractMissingVarName(err.Error())
		if v != "" {
			return "Missing variable " + v
		}
		return "Missing variable"
	}

	return "Unexpected error (see logs)"
}

func looksLikeYAMLProblem(s string) bool {
	ls := strings.ToLower(s)
	return strings.Contains(ls, "yaml:") || strings.Contains(ls, "did not find expected") || strings.Contains(ls, "cannot unmarshal")
}

func extractLine(s string) string {
	m := reLine.FindStringSubmatch(s)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func extractMissingVarName(s string) string {
	ls := strings.ToLower(s)

	i := strings.LastIndex(ls, "missing variable:")
	if i >= 0 {
		part := strings.TrimSpace(s[i+len("missing variable:"):])
		part = strings.Trim(part, " .,:;\"'")
		part = strings.Fields(part)[0]
		part = strings.Trim(part, " .,:;\"'")
		return part
	}

	i = strings.LastIndex(ls, "missing variable ")
	if i >= 0 {
		part := strings.TrimSpace(s[i+len("missing variable "):])
		part = strings.Trim(part, " .,:;\"'")
		part = strings.Fields(part)[0]
		part = strings.Trim(part, " .,:;\"'")
		return part
	}

	return ""
}
