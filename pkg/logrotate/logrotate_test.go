package logrotate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/logger"
	"go.uber.org/zap"
)

// silentLogger returns a no-op zap.SugaredLogger so test output stays clean.
func silentLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// stubUploader records every UploadFile call and can return a preset error.
type stubUploader struct {
	calls []uploadCall
	err   error
}

type uploadCall struct {
	objectName string
	filePath   string
}

func (s *stubUploader) UploadFile(_ context.Context, objectName, filePath string) error {
	s.calls = append(s.calls, uploadCall{objectName, filePath})
	return s.err
}

// makeRotator builds a LogRotator backed by a real temp-dir and RotatingWriter.
// up may be nil to test the no-MinIO path.
func makeRotator(t *testing.T, up Uploader) (*LogRotator, string) {
	t.Helper()
	dir := t.TempDir()
	_, rw := logger.NewDynamicLogger(config.LoggerConfig{
		Level: "debug",
		Dir:   dir,
	})
	r := &LogRotator{
		cron:   cron.New(),
		mc:     up,
		rw:     rw,
		logDir: dir,
		log:    silentLogger(),
	}
	return r, dir
}

// --- RotateToNewDay ---

func TestRotateToNewDay_CreatesFile(t *testing.T) {
	r, dir := makeRotator(t, nil)

	if err := r.RotateToNewDay(); err != nil {
		t.Fatalf("RotateToNewDay returned error: %v", err)
	}

	// RotateToNewDay creates the file for the NEXT day (now+1).
	expected := filepath.Join(dir, logger.LogFileName(time.Now().AddDate(0, 0, 1)))
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected rotated file %q to exist", expected)
	}
}

func TestRotateToNewDay_WritesGoToNewFile(t *testing.T) {
	r, dir := makeRotator(t, nil)

	if err := r.RotateToNewDay(); err != nil {
		t.Fatalf("RotateToNewDay: %v", err)
	}

	// Write through the RotatingWriter after rotation.
	r.rw.Write([]byte("rotated-content"))

	newFile := filepath.Join(dir, logger.LogFileName(time.Now().AddDate(0, 0, 1)))
	data, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("read rotated file: %v", err)
	}
	if string(data) != "rotated-content" {
		t.Errorf("rotated file = %q, want %q", string(data), "rotated-content")
	}
}

func TestRotateToNewDay_CreatesDirectoryIfMissing(t *testing.T) {
	r, dir := makeRotator(t, nil)
	// Point the rotator at a subdirectory that does not yet exist.
	r.logDir = filepath.Join(dir, "nested", "logs")

	if err := r.RotateToNewDay(); err != nil {
		t.Fatalf("RotateToNewDay should create missing dir, got: %v", err)
	}

	expected := filepath.Join(r.logDir, logger.LogFileName(time.Now().AddDate(0, 0, 1)))
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected file %q after dir auto-create", expected)
	}
}

// --- UploadToday ---

func TestUploadToday_NilClient_ReturnsNil(t *testing.T) {
	r, _ := makeRotator(t, nil)

	if err := r.UploadToday(context.Background()); err != nil {
		t.Errorf("UploadToday with nil client should return nil, got: %v", err)
	}
}

func TestUploadToday_CallsUploaderWithCorrectNames(t *testing.T) {
	stub := &stubUploader{}
	r, dir := makeRotator(t, stub)

	// Ensure the log file exists so the upload path is valid.
	logFile := filepath.Join(dir, logger.LogFileName(time.Now()))
	os.WriteFile(logFile, []byte("some logs"), 0o644)

	if err := r.UploadToday(context.Background()); err != nil {
		t.Fatalf("UploadToday returned error: %v", err)
	}

	if len(stub.calls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(stub.calls))
	}

	wantObject := logger.LogFileName(time.Now())
	if stub.calls[0].objectName != wantObject {
		t.Errorf("objectName = %q, want %q", stub.calls[0].objectName, wantObject)
	}
	if stub.calls[0].filePath != logFile {
		t.Errorf("filePath = %q, want %q", stub.calls[0].filePath, logFile)
	}
}

func TestUploadToday_PropagatesUploaderError(t *testing.T) {
	stub := &stubUploader{err: os.ErrPermission}
	r, dir := makeRotator(t, stub)

	logFile := filepath.Join(dir, logger.LogFileName(time.Now()))
	os.WriteFile(logFile, []byte("logs"), 0o644)

	if err := r.UploadToday(context.Background()); err == nil {
		t.Error("expected error from uploader to be propagated, got nil")
	}
}

// --- Upload then Rotate (combined flow) ---

func TestUploadSucceeds_ThenRotates(t *testing.T) {
	stub := &stubUploader{} // no error → upload succeeds
	r, dir := makeRotator(t, stub)

	logFile := filepath.Join(dir, logger.LogFileName(time.Now()))
	os.WriteFile(logFile, []byte("today logs"), 0o644)

	// Simulate the cron job: upload then rotate.
	ctx := context.Background()
	if err := r.UploadToday(ctx); err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if err := r.RotateToNewDay(); err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Upload was called exactly once.
	if len(stub.calls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(stub.calls))
	}

	// New file for tomorrow exists after rotation.
	newFile := filepath.Join(dir, logger.LogFileName(time.Now().AddDate(0, 0, 1)))
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Errorf("expected new log file %q to exist after rotation", newFile)
	}
}

func TestUploadFails_NoRotation(t *testing.T) {
	stub := &stubUploader{err: os.ErrPermission} // upload fails
	r, dir := makeRotator(t, stub)

	logFile := filepath.Join(dir, logger.LogFileName(time.Now()))
	os.WriteFile(logFile, []byte("today logs"), 0o644)

	ctx := context.Background()
	err := r.UploadToday(ctx)
	if err == nil {
		t.Fatal("expected upload to fail, got nil")
	}

	// Because upload failed, caller should NOT rotate — verify by writing
	// to the writer and checking the original file still receives the data.
	r.rw.Write([]byte("still-writing"))
	data, _ := os.ReadFile(logFile)
	if string(data) != "today logsstill-writing" {
		t.Errorf("expected original file to still receive writes, got: %q", string(data))
	}
}

// --- Start / Stop ---

func TestStartStop_NoPanic(t *testing.T) {
	r, _ := makeRotator(t, nil)
	// Start and Stop should not panic.
	r.Start()
	r.Stop()
}
