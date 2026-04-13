package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zeelrupapara/seo-rank-guardian/config"
)

// --- LogFileName ---

func TestLogFileName_Format(t *testing.T) {
	tt := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "april 11 2026",
			in:   time.Date(2026, time.April, 11, 0, 0, 0, 0, time.UTC),
			want: "logs-11:04:2026",
		},
		{
			name: "january 1",
			in:   time.Date(2025, time.January, 1, 12, 30, 0, 0, time.UTC),
			want: "logs-01:01:2025",
		},
		{
			name: "december 31",
			in:   time.Date(2024, time.December, 31, 23, 59, 59, 0, time.UTC),
			want: "logs-31:12:2024",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := LogFileName(tc.in)
			if got != tc.want {
				t.Errorf("LogFileName(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// --- RotatingWriter ---

// nopCloser wraps an io.Writer so it satisfies io.WriteCloser.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestRotatingWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	rw := &RotatingWriter{writer: nopCloser{&buf}}

	msg := "hello log\n"
	n, err := rw.Write([]byte(msg))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned n=%d, want %d", n, len(msg))
	}
	if buf.String() != msg {
		t.Errorf("buffer = %q, want %q", buf.String(), msg)
	}
}

func TestRotatingWriter_Swap(t *testing.T) {
	var first, second bytes.Buffer

	rw := &RotatingWriter{writer: nopCloser{&first}}

	// Write goes to first buffer.
	rw.Write([]byte("before"))

	// Swap to second buffer.
	if err := rw.Swap(nopCloser{&second}); err != nil {
		t.Fatalf("Swap returned error: %v", err)
	}

	// Write now goes to second buffer.
	rw.Write([]byte("after"))

	if first.String() != "before" {
		t.Errorf("first buffer = %q, want %q", first.String(), "before")
	}
	if second.String() != "after" {
		t.Errorf("second buffer = %q, want %q", second.String(), "after")
	}
}

func TestRotatingWriter_Sync(t *testing.T) {
	// Sync on a non-*os.File writer should return nil without panicking.
	var buf bytes.Buffer
	rw := &RotatingWriter{writer: nopCloser{&buf}}
	if err := rw.Sync(); err != nil {
		t.Errorf("Sync returned unexpected error: %v", err)
	}
}

// --- NewDynamicLogger ---

func TestNewDynamicLogger_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()

	cfg := config.LoggerConfig{
		Level: "debug",
		Dir:   dir,
	}

	log, rw := NewDynamicLogger(cfg)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
	if rw == nil {
		t.Fatal("expected non-nil RotatingWriter")
	}
	log.Sync()

	// A file named logs-DD:MM:YYYY should exist in the temp dir.
	expectedName := LogFileName(time.Now())
	expectedPath := filepath.Join(dir, expectedName)

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected log file %q to exist, but it does not", expectedPath)
	}
}

func TestNewDynamicLogger_WritesToFile(t *testing.T) {
	dir := t.TempDir()

	cfg := config.LoggerConfig{
		Level: "debug",
		Dir:   dir,
	}

	log, _ := NewDynamicLogger(cfg)
	log.Info("test-message-xyz")
	log.Sync()

	filePath := filepath.Join(dir, LogFileName(time.Now()))
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}
	if !strings.Contains(string(data), "test-message-xyz") {
		t.Errorf("log file does not contain expected message; got:\n%s", string(data))
	}
}
