package worker

import (
	"fmt"
	"strings"
)

func HandleJob(w *Worker, subject string, data []byte) error {
	// Route by subject prefix
	switch {
	case strings.HasPrefix(subject, "srg.jobs.scrape"):
		return w.handleScrapeTask(data)
	case strings.HasPrefix(subject, "srg.jobs.detect"):
		return w.handleChangeDetection(data)
	case strings.HasPrefix(subject, "srg.jobs.report"):
		return w.handleReportGeneration(data)
	default:
		w.Log.Warnf("No handler for subject: %s", subject)
		return fmt.Errorf("unknown job subject: %s", subject)
	}
}
