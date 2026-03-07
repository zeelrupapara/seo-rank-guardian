package model

import (
	"time"

	"gorm.io/gorm"
)

type CommonModel struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	CreatedBy uint           `json:"created_by"`
	UpdatedBy uint           `json:"updated_by"`
}
