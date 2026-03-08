package worker

import (
	"encoding/json"
	"fmt"

	"github.com/zeelrupapara/seo-rank-guardian/model"
)

type detectMessage struct {
	RunID uint `json:"run_id"`
	JobID uint `json:"job_id"`
}

func (w *Worker) handleChangeDetection(data []byte) error {
	var msg detectMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal detect message: %w", err)
	}

	w.Log.Infof("Running change detection for run %d, job %d", msg.RunID, msg.JobID)

	// Idempotency: skip if diffs already exist for this run
	var existingDiffs int64
	w.DB.Model(&model.RankDiff{}).Where("run_id = ?", msg.RunID).Count(&existingDiffs)
	if existingDiffs > 0 {
		w.Log.Infof("Diffs already exist for run %d, skipping detection", msg.RunID)
		w.triggerReport(msg.RunID, msg.JobID)
		return nil
	}

	// Emit detect_started event
	w.publishEvent(model.RunEvent{
		Type:  model.EventDetectStarted,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.DetectEventPayload{
			Message: "Change detection started",
		},
	})

	// Find previous run
	var prevRun model.JobRun
	err := w.DB.Where("job_id = ? AND id < ? AND status IN ?", msg.JobID, msg.RunID, []string{"completed", "partial"}).
		Order("id DESC").First(&prevRun).Error
	if err != nil {
		w.Log.Infof("No previous run found for job %d, skipping change detection", msg.JobID)
		w.triggerReport(msg.RunID, msg.JobID)
		return nil
	}

	// Get current results grouped by keyword+state+domain
	var currentResults []model.SearchResult
	w.DB.Where("run_id = ? AND (is_target = ? OR is_competitor = ?)", msg.RunID, true, true).Find(&currentResults)

	// Get previous results
	var prevResults []model.SearchResult
	w.DB.Where("run_id = ? AND (is_target = ? OR is_competitor = ?)", prevRun.ID, true, true).Find(&prevResults)

	// Build lookup: keyword|state|domain -> position
	prevPositions := make(map[string]int)
	for _, r := range prevResults {
		key := fmt.Sprintf("%s|%s|%s", r.Keyword, r.State, r.Domain)
		prevPositions[key] = r.Position
	}

	currPositions := make(map[string]int)
	for _, r := range currentResults {
		key := fmt.Sprintf("%s|%s|%s", r.Keyword, r.State, r.Domain)
		currPositions[key] = r.Position
	}

	// Track change counts for event
	var improvedCount, droppedCount, newCount, lostCount int

	// Detect changes for current results
	for _, r := range currentResults {
		key := fmt.Sprintf("%s|%s|%s", r.Keyword, r.State, r.Domain)
		prevPos := prevPositions[key]
		currPos := r.Position

		var changeType string
		var delta int
		if prevPos == 0 {
			changeType = "new"
			delta = 0
			newCount++
		} else {
			delta = currPos - prevPos
			if delta < 0 {
				changeType = "improved"
				improvedCount++
			} else if delta > 0 {
				changeType = "dropped"
				droppedCount++
			} else {
				continue // no change
			}
		}

		diff := model.RankDiff{
			JobID:        msg.JobID,
			RunID:        msg.RunID,
			PrevRunID:    prevRun.ID,
			Domain:       r.Domain,
			PrevPosition: prevPos,
			CurrPosition: currPos,
			Delta:        delta,
			ChangeType:   changeType,
			Keyword:      r.Keyword,
			State:        r.State,
		}

		if err := w.DB.Create(&diff).Error; err != nil {
			w.Log.Errorf("Failed to create rank diff: %v", err)
		}
	}

	// Detect "lost" -- was in previous but not in current
	for _, r := range prevResults {
		key := fmt.Sprintf("%s|%s|%s", r.Keyword, r.State, r.Domain)
		if _, found := currPositions[key]; !found {
			lostCount++
			diff := model.RankDiff{
				JobID:        msg.JobID,
				RunID:        msg.RunID,
				PrevRunID:    prevRun.ID,
				Domain:       r.Domain,
				PrevPosition: r.Position,
				CurrPosition: 0,
				Delta:        r.Position, // positive delta = dropped off (was at position N, now gone)
				ChangeType:   "lost",
				Keyword:      r.Keyword,
				State:        r.State,
			}
			if err := w.DB.Create(&diff).Error; err != nil {
				w.Log.Errorf("Failed to create rank diff (lost): %v", err)
			}
		}
	}

	w.Log.Infof("Change detection completed for run %d", msg.RunID)

	// Emit detect_complete event
	w.publishEvent(model.RunEvent{
		Type:  model.EventDetectComplete,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.DetectEventPayload{
			Message:  "Change detection complete",
			Improved: improvedCount,
			Dropped:  droppedCount,
			New:      newCount,
			Lost:     lostCount,
		},
	})

	w.triggerReport(msg.RunID, msg.JobID)
	return nil
}

func (w *Worker) triggerReport(runID, jobID uint) {
	reportMsg, err := json.Marshal(map[string]interface{}{
		"run_id": runID,
		"job_id": jobID,
	})
	if err != nil {
		w.Log.Errorf("Failed to marshal report message: %v", err)
		return
	}
	if err := w.Nats.Publish("srg.jobs.report", reportMsg); err != nil {
		w.Log.Errorf("Failed to publish report job: %v", err)
	}
}
