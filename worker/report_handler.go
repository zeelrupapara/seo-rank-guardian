package worker

import (
	"encoding/json"
	"fmt"

	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/ai"
)

type reportMessage struct {
	RunID uint `json:"run_id"`
	JobID uint `json:"job_id"`
}

func (w *Worker) handleReportGeneration(data []byte) error {
	var msg reportMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal report message: %w", err)
	}

	w.Log.Infof("Generating report for run %d, job %d", msg.RunID, msg.JobID)

	// Emit report_started event
	w.publishEvent(model.RunEvent{
		Type:  model.EventReportStarted,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.ReportEventPayload{
			Message: "Report generation started",
		},
	})

	// Get job details (Unscoped to handle soft-deleted jobs)
	var job model.Job
	if err := w.DB.Unscoped().First(&job, msg.JobID).Error; err != nil {
		w.Log.Warnf("Job %d not found, skipping report generation: %v", msg.JobID, err)
		return nil
	}

	// Get all results for this run
	var results []model.SearchResult
	w.DB.Where("run_id = ?", msg.RunID).Order("keyword, state, position").Find(&results)

	// Get diffs for this run
	var diffs []model.RankDiff
	w.DB.Where("run_id = ?", msg.RunID).Find(&diffs)

	// Check if AI client is available
	if w.AI == nil {
		w.Log.Warn("AI client not configured, skipping report generation")
		w.publishEvent(model.RunEvent{
			Type:  model.EventReportFailed,
			RunID: msg.RunID,
			JobID: msg.JobID,
			Payload: model.ReportEventPayload{
				Message: "Report generation skipped",
				Error:   "AI client not configured",
			},
		})
		return nil
	}

	// Idempotency: skip if a report already exists for this run
	var existingReport model.Report
	if err := w.DB.Where("run_id = ?", msg.RunID).
		First(&existingReport).Error; err == nil {
		w.Log.Infof("Report already exists for run %d (status: %s), skipping", msg.RunID, existingReport.Status)
		return nil
	}

	// Convert model types to ai prompt types
	reportResults := make([]ai.ReportResult, len(results))
	for i, r := range results {
		reportResults[i] = ai.ReportResult{
			Keyword:      r.Keyword,
			State:        r.State,
			Position:     r.Position,
			Domain:       r.Domain,
			URL:          r.URL,
			Title:        r.Title,
			Snippet:      r.Snippet,
			IsTarget:     r.IsTarget,
			IsCompetitor: r.IsCompetitor,
		}
	}

	reportDiffs := make([]ai.ReportDiff, len(diffs))
	for i, d := range diffs {
		reportDiffs[i] = ai.ReportDiff{
			Domain:       d.Domain,
			Keyword:      d.Keyword,
			State:        d.State,
			ChangeType:   d.ChangeType,
			PrevPosition: d.PrevPosition,
			CurrPosition: d.CurrPosition,
			Delta:        d.Delta,
		}
	}

	userContent := ai.BuildReportContent(job.Domain, job.GetCompetitors(), reportResults, reportDiffs)

	// Create report record
	report := model.Report{
		JobID:    msg.JobID,
		RunID:    msg.RunID,
		Provider: w.Cfg.AI.Provider,
		Model:    w.Cfg.AI.Model,
		Prompt:   userContent,
		Status:   "generating",
	}
	if err := w.DB.Create(&report).Error; err != nil {
		return fmt.Errorf("failed to create report record: %w", err)
	}

	// Call AI with structured output
	aiResult, err := w.AI.AnalyzeStructured(ai.AnalyzeOptions{
		SystemInstruction: ai.SEOSystemInstruction,
		UserContent:       userContent,
		ResponseSchema:    ai.ReportSchema,
		EnableSearch:      w.Cfg.AI.SearchGrounding,
	})
	if err != nil {
		w.Log.Errorf("AI analysis failed for run %d: %v", msg.RunID, err)
		w.DB.Model(&report).Updates(map[string]interface{}{
			"status": "failed",
		})

		w.publishEvent(model.RunEvent{
			Type:  model.EventReportFailed,
			RunID: msg.RunID,
			JobID: msg.JobID,
			Payload: model.ReportEventPayload{
				Message: "Report generation failed",
				Error:   "AI analysis failed",
			},
		})

		// Return nil to ACK the message — failure is recorded in DB, don't redeliver
		return nil
	}

	// Structured output guarantees valid JSON — no fallback needed
	w.DB.Model(&report).Updates(map[string]interface{}{
		"result":         json.RawMessage(aiResult.Content),
		"grounding_meta": aiResult.GroundingMeta,
		"status":         "generated",
	})

	// Emit report_complete event
	w.publishEvent(model.RunEvent{
		Type:  model.EventReportComplete,
		RunID: msg.RunID,
		JobID: msg.JobID,
		Payload: model.ReportEventPayload{
			Message: "AI report generated",
		},
	})

	w.Log.Infof("Report generated for run %d", msg.RunID)
	return nil
}
