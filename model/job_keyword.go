package model

type JobKeyword struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime:nano"`
	JobID     uint   `json:"job_id" gorm:"index;not null"`
	Keyword   string `json:"keyword" gorm:"size:500;not null"`
}

func (JobKeyword) TableName() string { return "srg_job_keywords" }
