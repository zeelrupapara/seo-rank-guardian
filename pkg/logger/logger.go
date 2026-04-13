package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RotatingWriter is a thread-safe io.WriteCloser whose underlying writer can
// be swapped atomically. Pass it to zapcore so the logger writes to the
// current day's file without needing to recreate the logger.
type RotatingWriter struct {
	mu     sync.Mutex
	writer io.WriteCloser
}

func (rw *RotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.writer.Write(p)
}

func (rw *RotatingWriter) Sync() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if f, ok := rw.writer.(*os.File); ok {
		return f.Sync()
	}
	return nil
}

// Swap closes the current writer and replaces it with newWriter.
func (rw *RotatingWriter) Swap(newWriter io.WriteCloser) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if err := rw.writer.Close(); err != nil {
		return err
	}
	rw.writer = newWriter
	return nil
}

// LogFileName returns the daily log filename for the given time.
// Format: logs-DD:MM:YYYY
func LogFileName(t time.Time) string {
	return fmt.Sprintf("logs-%02d:%02d:%d", t.Day(), int(t.Month()), t.Year())
}

// NewDynamicLogger creates a Zap logger backed by a RotatingWriter.
// The initial log file is named after today's date (logs-DD:MM:YYYY-00:00:00)
// and written into cfg.Dir. The returned RotatingWriter can be used by the
// log rotator to swap the file at midnight.
func NewDynamicLogger(cfg config.LoggerConfig) (*zap.SugaredLogger, *RotatingWriter) {
	level := parseLevel(cfg.Level)

	// Ensure log directory exists.
	_ = os.MkdirAll(cfg.Dir, 0o755)

	filePath := cfg.Dir + "/" + LogFileName(time.Now())
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// Fall back to stderr if the file cannot be opened.
		f = os.Stderr
	}

	rw := &RotatingWriter{writer: f}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(rw), level),
		zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(os.Stdout), level),
	)

	return zap.New(core).Sugar(), rw
}

// NewLogger is kept for backward compatibility; it delegates to NewDynamicLogger.
func NewLogger(cfg config.LoggerConfig) *zap.SugaredLogger {
	log, _ := NewDynamicLogger(cfg)
	return log
}

func parseLevel(l string) zapcore.Level {
	switch l {
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.DebugLevel
	}
}
