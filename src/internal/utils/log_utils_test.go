package utils

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetRootLoggerToFileWritesLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xdfile.log")
	if err := SetRootLoggerToFile(path, false); err != nil {
		t.Fatalf("set root logger to file: %v", err)
	}
	t.Cleanup(SetRootLoggerToDiscarded)

	slog.Info("test log entry", "kind", "file")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "test log entry") {
		t.Fatalf("expected log file to contain test entry, got %q", string(data))
	}
}
