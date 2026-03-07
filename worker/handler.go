package worker

import "fmt"

func HandleJob(w *Worker, subject string, data []byte) error {
	switch subject {
	default:
		w.Log.Warnf("No handler for subject: %s", subject)
		return fmt.Errorf("unknown job subject: %s", subject)
	}
}
