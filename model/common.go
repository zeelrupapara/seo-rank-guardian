package model

import "gorm.io/plugin/soft_delete"

type CommonModel struct {
	ID        uint                  `json:"id" gorm:"primaryKey"`
	CreatedAt int64                 `json:"created_at" gorm:"autoCreateTime:nano"`
	UpdatedAt int64                 `json:"updated_at" gorm:"autoUpdateTime:nano"`
	DeletedAt soft_delete.DeletedAt `json:"deleted_at,omitempty" gorm:"index;softDelete:nano"`
	CreatedBy uint                  `json:"created_by"`
	UpdatedBy uint                  `json:"updated_by"`
}
