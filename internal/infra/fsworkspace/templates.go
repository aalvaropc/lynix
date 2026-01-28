package fsworkspace

import "embed"

//go:embed templates/**/*
var templatesFS embed.FS
