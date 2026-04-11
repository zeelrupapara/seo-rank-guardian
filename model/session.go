package model

import "time"

// Session represents a user login session.
// Uses a string primary key (hex) instead of uint to prevent enumeration attacks.
// RevokedAt serves as the revocation marker — no soft-delete needed.
type Session struct {
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	UserID       uint       `json:"user_id" gorm:"index;not null"`
	IPAddress    string     `json:"ip_address" gorm:"size:64"`
	UserAgent    string     `json:"user_agent" gorm:"size:500"`
	DeviceInfo   string     `json:"device_info" gorm:"size:200"` // parsed: "Chrome on macOS"
	LoginMethod  string     `json:"login_method" gorm:"size:50"` // "password" | "google"
	CreatedAt    int64      `json:"created_at" gorm:"autoCreateTime:nano"`
	LastActiveAt int64      `json:"last_active_at"`
	ExpiresAt    int64      `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    uint       `json:"revoked_by,omitempty"` // 0 = self-logout, >0 = admin userID
}

func (Session) TableName() string { return "srg_sessions" }
