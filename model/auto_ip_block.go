package model

import "time"

// AutoIPBlock tracks IPs that have been automatically blocked due to rate limit violations.
// One row per unique IP address — updated (upserted) each time the IP is blocked.
type AutoIPBlock struct {
	CommonModel
	IPAddress    string     `json:"ip_address"   gorm:"size:50;not null;uniqueIndex"`
	BlockLevel   int        `json:"block_level"`               // 1=5min, 2=10min, 3=1month
	BlockedUntil time.Time  `json:"blocked_until"`
	L1Count      int        `json:"l1_count"`                  // cumulative L1 blocks for this IP
	L2Count      int        `json:"l2_count"`                  // cumulative L2 blocks for this IP
	IsActive     bool       `json:"is_active"    gorm:"default:true;not null"`
	UnblockedAt  *time.Time `json:"unblocked_at"`
	UnblockedBy  uint       `json:"unblocked_by"`
}

func (AutoIPBlock) TableName() string { return "srg_auto_ip_blocks" }
