package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/scraper"
	"gorm.io/gorm"
)

type scrapeMessage struct {
	PairID      uint     `json:"pair_id"`
	RunID       uint     `json:"run_id"`
	JobID       uint     `json:"job_id"`
	SearchQuery string   `json:"search_query"`
	Keyword     string   `json:"keyword"`
	State       string   `json:"state"`
	Country     string   `json:"country"`
	Domain      string   `json:"domain"`
	Competitors []string `json:"competitors"`
}

func (w *Worker) handleScrapeTask(data []byte) error {
	var msg scrapeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal scrape message: %w", err)
	}

	w.Log.Infof("Processing scrape pair %d: %s", msg.PairID, msg.SearchQuery)

	// Idempotency: check if pair is already completed/failed before doing anything
	var existingPair model.SearchPair
	if err := w.DB.First(&existingPair, msg.PairID).Error; err == nil {
		if existingPair.Status == "completed" || existingPair.Status == "failed" {
			w.Log.Infof("Pair %d already %s, skipping", msg.PairID, existingPair.Status)
			return nil
		}
	}

	// Emit scrape_started event
	w.publishEvent(model.RunEvent{
		Type:  model.EventScrapeStarted,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.ScrapeEventPayload{
			PairID:  msg.PairID,
			Keyword: msg.Keyword,
			State:   msg.State,
			Message: "Scraping started",
		},
	})

	// Mark pair as running
	now := time.Now()
	w.DB.Model(&model.SearchPair{}).Where("id = ?", msg.PairID).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": now,
	})

	// Scrape Google
	results, err := w.Scraper.Search(scraper.SearchOptions{
		Query:       msg.SearchQuery,
		Region:      strings.ToLower(msg.Country),
		Language:    "en",
		ResultLimit: w.Cfg.Scraper.ResultLimit,
	})
	if err != nil {
		w.Log.Errorf("Scrape failed for pair %d: %v", msg.PairID, err)
		finishedAt := time.Now()
		if dbErr := w.DB.Model(&model.SearchPair{}).Where("id = ?", msg.PairID).Updates(map[string]interface{}{
			"status":      "failed",
			"error_msg":   err.Error(),
			"finished_at": finishedAt,
		}).Error; dbErr != nil {
			w.Log.Errorf("Failed to update pair status: %v", dbErr)
		}
		if dbErr := w.DB.Model(&model.JobRun{}).Where("id = ?", msg.RunID).
			Update("failed_pairs", gorm.Expr("failed_pairs + 1")).Error; dbErr != nil {
			w.Log.Errorf("Failed to increment failed_pairs: %v", dbErr)
		}

		// Emit scrape_failed event
		w.publishEvent(model.RunEvent{
			Type:  model.EventScrapeFailed,
			RunID: msg.RunID,
			JobID: msg.JobID,
			Payload: model.ScrapeEventPayload{
				PairID:  msg.PairID,
				Keyword: msg.Keyword,
				State:   msg.State,
				Message: "Scrape failed",
				Error:   err.Error(),
			},
		})

		w.checkRunCompletion(msg.RunID, msg.JobID)
		// Return nil to ACK the message — side effects are already committed
		return nil
	}

	// Store results
	targetDomain := strings.TrimPrefix(strings.TrimPrefix(msg.Domain, "https://"), "http://")
	targetDomain = strings.TrimPrefix(targetDomain, "www.")

	for _, r := range results {
		cleanDomain := strings.TrimPrefix(r.Domain, "www.")

		isTarget := strings.EqualFold(cleanDomain, targetDomain)
		isCompetitor := false
		for _, comp := range msg.Competitors {
			cleanComp := strings.TrimPrefix(strings.TrimPrefix(comp, "https://"), "http://")
			cleanComp = strings.TrimPrefix(cleanComp, "www.")
			if strings.EqualFold(cleanDomain, cleanComp) {
				isCompetitor = true
				break
			}
		}

		result := model.SearchResult{
			PairID:       msg.PairID,
			RunID:        msg.RunID,
			JobID:        msg.JobID,
			Domain:       r.Domain,
			Position:     r.Position,
			URL:          r.URL,
			Title:        r.Title,
			Snippet:      r.Snippet,
			IsTarget:     isTarget,
			IsCompetitor: isCompetitor,
			Keyword:      msg.Keyword,
			State:        msg.State,
		}

		if err := w.DB.Create(&result).Error; err != nil {
			w.Log.Errorf("Failed to store result: %v", err)
		}
	}

	// Mark pair as completed
	finishedAt := time.Now()
	if dbErr := w.DB.Model(&model.SearchPair{}).Where("id = ?", msg.PairID).Updates(map[string]interface{}{
		"status":      "completed",
		"finished_at": finishedAt,
	}).Error; dbErr != nil {
		w.Log.Errorf("Failed to update pair status: %v", dbErr)
	}

	// Update run progress
	if dbErr := w.DB.Model(&model.JobRun{}).Where("id = ?", msg.RunID).
		Update("completed_pairs", gorm.Expr("completed_pairs + 1")).Error; dbErr != nil {
		w.Log.Errorf("Failed to increment completed_pairs: %v", dbErr)
	}

	// Find target position for this pair
	var targetPosition int
	for _, r := range results {
		cleanDomain := strings.TrimPrefix(r.Domain, "www.")
		if strings.EqualFold(cleanDomain, targetDomain) {
			targetPosition = r.Position
			break
		}
	}

	// Emit scrape_complete event
	w.publishEvent(model.RunEvent{
		Type:  model.EventScrapeComplete,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.ScrapeEventPayload{
			PairID:      msg.PairID,
			Keyword:     msg.Keyword,
			State:       msg.State,
			Message:     "Scrape complete",
			Position:    targetPosition,
			ResultCount: len(results),
			Domain:      msg.Domain,
		},
	})

	// Emit run_progress event
	var updatedRun model.JobRun
	w.DB.First(&updatedRun, msg.RunID)
	w.publishEvent(model.RunEvent{
		Type:  model.EventRunProgress,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.ProgressEventPayload{
			Message:        "Progress",
			CompletedPairs: updatedRun.CompletedPairs,
			TotalPairs:     updatedRun.TotalPairs,
		},
	})

	w.Log.Infof("Completed scrape pair %d: %d results", msg.PairID, len(results))

	w.checkRunCompletion(msg.RunID, msg.JobID)
	return nil
}

func (w *Worker) checkRunCompletion(runID, jobID uint) {
	var run model.JobRun
	if err := w.DB.First(&run, runID).Error; err != nil {
		w.Log.Errorf("Failed to load run %d for completion check: %v", runID, err)
		return
	}

	if run.CompletedPairs+run.FailedPairs >= run.TotalPairs {
		now := time.Now()
		status := "completed"
		if run.FailedPairs > 0 && run.CompletedPairs == 0 {
			status = "failed"
		} else if run.FailedPairs > 0 {
			status = "partial"
		}

		if dbErr := w.DB.Model(&run).Updates(map[string]interface{}{
			"status":       status,
			"completed_at": now,
		}).Error; dbErr != nil {
			w.Log.Errorf("Failed to update run %d status: %v", runID, dbErr)
		}

		w.Log.Infof("Run %d finished with status: %s (completed: %d, failed: %d)",
			runID, status, run.CompletedPairs, run.FailedPairs)

		// Emit run_complete or run_failed event
		if status == "failed" {
			w.publishEvent(model.RunEvent{
				Type:  model.EventRunFailed,
				RunID: runID,
				JobID: jobID,
				Payload: model.RunStatusEventPayload{
					Message:    "Run failed",
					TotalPairs: run.TotalPairs,
				},
			})
			// Do NOT trigger detect/report for fully failed runs
			return
		}

		w.publishEvent(model.RunEvent{
			Type:  model.EventRunComplete,
			RunID: runID,
			JobID: jobID,
			Payload: model.RunStatusEventPayload{
				Message:    fmt.Sprintf("Run completed with status: %s", status),
				TotalPairs: run.TotalPairs,
			},
		})

		// Trigger change detection only for successful/partial runs
		detectMsg, err := json.Marshal(map[string]interface{}{
			"run_id": runID,
			"job_id": jobID,
		})
		if err != nil {
			w.Log.Errorf("Failed to marshal detect message: %v", err)
			return
		}
		if err := w.Nats.Publish("srg.jobs.detect", detectMsg); err != nil {
			w.Log.Errorf("Failed to publish detect job: %v", err)
		}
	}
}
