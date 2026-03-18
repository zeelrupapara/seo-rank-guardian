package model

import (
	"gorm.io/datatypes"
)

// --- Event envelope (used for NATS + SSE transport) ---

type RunEventType string

const (
	EventRunStarted     RunEventType = "run_started"
	EventScrapeStarted  RunEventType = "scrape_started"
	EventScrapeComplete RunEventType = "scrape_complete"
	EventScrapeFailed   RunEventType = "scrape_failed"
	EventRunProgress    RunEventType = "run_progress"
	EventDetectStarted  RunEventType = "detect_started"
	EventDetectComplete RunEventType = "detect_complete"
	EventReportStarted  RunEventType = "report_started"
	EventReportComplete RunEventType = "report_complete"
	EventReportFailed   RunEventType = "report_failed"
	EventScrapeRetry    RunEventType = "scrape_retry"
	EventScrapeFallback RunEventType = "scrape_fallback"
	EventRunComplete    RunEventType = "run_complete"
	EventRunFailed      RunEventType = "run_failed"
)

type RunEvent struct {
	Type      RunEventType `json:"type"`
	RunID     uint         `json:"run_id"`
	JobID     uint         `json:"job_id"`
	Timestamp int64        `json:"timestamp"`
	Payload   interface{}  `json:"payload"`
}

// --- Typed payloads ---

type ScrapeEventPayload struct {
	PairID      uint   `json:"pair_id"`
	Keyword     string `json:"keyword"`
	State       string `json:"state"`
	Message     string `json:"message"`
	Position    int    `json:"position,omitempty"`
	ResultCount int    `json:"result_count,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ProgressEventPayload struct {
	Message        string `json:"message"`
	CompletedPairs int    `json:"completed_pairs"`
	TotalPairs     int    `json:"total_pairs"`
}

type DetectEventPayload struct {
	Message  string `json:"message"`
	Improved int    `json:"improved,omitempty"`
	Dropped  int    `json:"dropped,omitempty"`
	New      int    `json:"new,omitempty"`
	Lost     int    `json:"lost,omitempty"`
}

type ReportEventPayload struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type RunStatusEventPayload struct {
	Message    string `json:"message"`
	TotalPairs int    `json:"total_pairs,omitempty"`
	Error      string `json:"error,omitempty"`
}

// --- DB model for persistence (append-only, no soft-delete) ---

type RunEventLog struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt int64          `json:"created_at" gorm:"autoCreateTime:nano"`
	RunID     uint           `json:"run_id" gorm:"index;not null"`
	JobID     uint           `json:"job_id" gorm:"index;not null"`
	EventType RunEventType   `json:"event_type" gorm:"size:50;not null;index"`
	Data      datatypes.JSON `json:"data" gorm:"type:jsonb;not null"`
}

func (RunEventLog) TableName() string { return "srg_run_events" }
