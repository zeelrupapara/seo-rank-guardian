package model

type JobRun struct {
	CommonModel
	JobID          uint   `json:"job_id" gorm:"index;not null"`
	Status         string `json:"status" gorm:"size:50;default:pending"`
	TotalPairs     int    `json:"total_pairs"`
	CompletedPairs int    `json:"completed_pairs" gorm:"default:0"`
	FailedPairs    int    `json:"failed_pairs" gorm:"default:0"`
	StartedAt      *int64 `json:"started_at"`
	CompletedAt    *int64 `json:"completed_at"`
	TriggeredBy    string `json:"triggered_by" gorm:"size:50"`
}

func (JobRun) TableName() string { return "srg_job_runs" }
