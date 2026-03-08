package model

import "time"

type SearchPair struct {
	CommonModel
	RunID       uint       `json:"run_id" gorm:"index;not null"`
	JobID       uint       `json:"job_id" gorm:"index"`
	Status      string     `json:"status" gorm:"size:50;default:pending"`
	ErrorMsg    string     `json:"error_msg,omitempty" gorm:"type:text"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	Keyword     string     `json:"keyword" gorm:"size:500"`
	State       string     `json:"state" gorm:"size:255"`
	Country     string     `json:"country" gorm:"size:100"`
	SearchQuery string     `json:"search_query" gorm:"size:1000"`
}

func (SearchPair) TableName() string { return "srg_search_pairs" }
