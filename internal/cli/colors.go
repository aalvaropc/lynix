package cli

import (
	"io"
	"os"
)

// palette holds ANSI escape sequences; every field is empty when color is off.
type palette struct {
	red    string
	green  string
	yellow string
	dim    string
	bold   string
	reset  string
}

func newPalette(enabled bool) palette {
	if !enabled {
		return palette{}
	}
	return palette{
		red:    "\033[31m",
		green:  "\033[32m",
		yellow: "\033[33m",
		dim:    "\033[2m",
		bold:   "\033[1m",
		reset:  "\033[0m",
	}
}

// colorsEnabled decides whether to emit ANSI colors: an explicit --no-color,
// the NO_COLOR convention, dumb terminals, and non-TTY writers all disable it.
func colorsEnabled(noColorFlag bool, w io.Writer) bool {
	if noColorFlag {
		return false
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isTerminal reports whether the writer is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
