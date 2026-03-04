package logger

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Root  string
	Debug bool
}

var (
	mu       sync.Mutex // protects logFile, logPath, initedAt (write-time state)
	globalP  atomic.Pointer[slog.Logger]
	logFile  *os.File
	logPath  string
	initedAt time.Time
)

func init() {
	discard := slog.New(slog.NewJSONHandler(io.Discard, nil))
	globalP.Store(discard)
}

func Setup(cfg Config) (func() error, error) {
	root := filepath.Clean(cfg.Root)
	if root == "" {
		root = "."
	}

	dir := filepath.Join(root, ".lynix", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		setDiscard()
		return nil, err
	}

	path := filepath.Join(dir, "lynix.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		setDiscard()
		return nil, err
	}

	level := slog.LevelInfo
	addSource := false
	if cfg.Debug {
		level = slog.LevelDebug
		addSource = true
	}

	h := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {

			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				t := a.Value.Time().UTC()
				a.Value = slog.StringValue(t.Format(time.RFC3339Nano))
			}
			return a
		},
	})

	l := slog.New(h)

	mu.Lock()
	logFile = f
	logPath = path
	initedAt = time.Now().UTC()
	mu.Unlock()

	globalP.Store(l)
	l.Info("logger.initialized", "path", path, "debug", cfg.Debug)

	cleanup := func() error {
		mu.Lock()
		defer mu.Unlock()

		var cerr error
		if logFile != nil {
			cerr = logFile.Close()
		}
		logFile = nil
		logPath = ""
		initedAt = time.Time{}
		discard := slog.New(slog.NewJSONHandler(io.Discard, nil))
		globalP.Store(discard)
		return cerr
	}

	return cleanup, nil
}

// L returns the global logger. Safe for concurrent use without locks.
func L() *slog.Logger {
	return globalP.Load()
}

func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

func InitTime() time.Time {
	mu.Lock()
	defer mu.Unlock()
	return initedAt
}

func setDiscard() {
	mu.Lock()
	defer mu.Unlock()
	discard := slog.New(slog.NewJSONHandler(io.Discard, nil))
	globalP.Store(discard)
	logFile = nil
	logPath = ""
	initedAt = time.Time{}
}

func IsReady() error {
	mu.Lock()
	defer mu.Unlock()
	if logFile == nil || logPath == "" {
		return errors.New("logger not initialized")
	}
	return nil
}
