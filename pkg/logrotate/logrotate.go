package logrotate

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/logger"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/minio"
	"go.uber.org/zap"
)

// Uploader is the upload capability required by LogRotator.
// *minio.Client satisfies this interface.
type Uploader interface {
	UploadFile(ctx context.Context, objectName, filePath string) error
}

// LogRotator manages daily log file rotation and MinIO uploads.
//
// Schedule (single job at 23:59):
//   1. Upload today's log file to MinIO
//   2. Only if upload succeeds → create tomorrow's file and swap the writer
//   3. If upload fails → log error, keep writing to the current file
type LogRotator struct {
	cron   *cron.Cron
	mc     Uploader // may be nil if MinIO is unavailable
	rw     *logger.RotatingWriter
	logDir string
	log    *zap.SugaredLogger
}

// New creates a LogRotator. mc may be nil; uploads are silently skipped if so.
func New(mc *minio.Client, rw *logger.RotatingWriter, logDir string, log *zap.SugaredLogger) *LogRotator {
	var up Uploader
	if mc != nil {
		up = mc
	}
	return &LogRotator{
		cron:   cron.New(),
		mc:     up,
		rw:     rw,
		logDir: logDir,
		log:    log,
	}
}

// Start registers the cron job and begins the scheduler.
// At 23:59 every day it uploads the current log file to MinIO.
// Only if the upload succeeds does it rotate to a new log file for the next day.
func (r *LogRotator) Start() {
	r.cron.AddFunc("59 23 * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := r.UploadToday(ctx); err != nil {
			r.log.Errorf("LogRotator: upload failed, skipping rotation: %v", err)
			return
		}

		if err := r.RotateToNewDay(); err != nil {
			r.log.Errorf("LogRotator: rotation failed: %v", err)
		}
	})

	r.cron.Start()
	r.log.Infof("LogRotator: started (logDir=%s)", r.logDir)
}

// Stop gracefully stops the cron scheduler.
func (r *LogRotator) Stop() {
	r.cron.Stop()
}

// UploadToday uploads the current day's log file to MinIO.
// It is safe to call manually for testing.
func (r *LogRotator) UploadToday(ctx context.Context) error {
	if r.mc == nil {
		r.log.Warn("LogRotator: MinIO client not configured, skipping upload")
		return nil
	}
	name := logger.LogFileName(time.Now())
	filePath := filepath.Join(r.logDir, name)
	r.log.Infof("LogRotator: uploading %s to MinIO", filePath)
	return r.mc.UploadFile(ctx, name, filePath)
}

// RotateToNewDay opens a log file for the next day and swaps the RotatingWriter.
// Called at 23:59, so time.Now() is still today — we add 1 day to get tomorrow's name.
// It is safe to call manually for testing.
func (r *LogRotator) RotateToNewDay() error {
	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		return err
	}
	name := logger.LogFileName(time.Now().AddDate(0, 0, 1))
	filePath := filepath.Join(r.logDir, name)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := r.rw.Swap(f); err != nil {
		f.Close()
		return err
	}
	r.log.Infof("LogRotator: rotated to new log file %s", filePath)
	return nil
}
