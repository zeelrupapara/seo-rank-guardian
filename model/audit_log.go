package model

import "gorm.io/datatypes"

// AuditLog is an append-only record of every mutating action in the system.
// Rows are never soft-deleted; Username is denormalized so history is preserved
// even if the user account is later removed.
type AuditLog struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	CreatedAt  int64          `json:"created_at" gorm:"autoCreateTime:nano;index"`
	UserID     uint           `json:"user_id" gorm:"index"`
	Username   string         `json:"username" gorm:"size:255"`
	Action     string         `json:"action" gorm:"size:100;index"` // e.g. "admin.update_role"
	Resource   string         `json:"resource" gorm:"size:50;index"` // "user", "job", "session", "policy"
	ResourceID string         `json:"resource_id" gorm:"size:100"`   // ID of the affected entity
	IPAddress  string         `json:"ip_address" gorm:"size:64"`
	Meta       datatypes.JSON `json:"meta" gorm:"type:jsonb"` // action-specific context
}

func (AuditLog) TableName() string { return "srg_audit_logs" }
