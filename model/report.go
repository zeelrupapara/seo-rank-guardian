package model

import "gorm.io/datatypes"

type Report struct {
	CommonModel
	JobID         uint           `json:"job_id" gorm:"index;not null"`
	RunID         uint           `json:"run_id" gorm:"index;not null"`
	Provider      string         `json:"provider" gorm:"size:50"`
	Model         string         `json:"model" gorm:"size:100"`
	Prompt        string         `json:"prompt" gorm:"type:text"`
	Result        datatypes.JSON `json:"result" swaggertype:"object"`
	GroundingMeta datatypes.JSON `json:"grounding_meta,omitempty" swaggertype:"object"`
	Status        string         `json:"status" gorm:"size:50;default:pending"`
}

func (Report) TableName() string { return "srg_reports" }
