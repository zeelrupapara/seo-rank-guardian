package model

// RequestLog is an append-only table — no soft-delete, no CommonModel overhead.
// One row per HTTP request that passes bot detection.
type RequestLog struct {
	ID           uint   `json:"id"            gorm:"primaryKey"`
	CreatedAt    int64  `json:"created_at"    gorm:"autoCreateTime:nano;index"`
	Method       string `json:"method"        gorm:"size:10;not null"`
	RoutePattern string `json:"route_pattern" gorm:"size:255;not null;index"` // Fiber route template, e.g. /api/v1/jobs/:jobId/rankings
	StatusCode   int    `json:"status_code"   gorm:"not null;index"`
	LatencyMs    int64  `json:"latency_ms"    gorm:"not null"`
	UserID       uint   `json:"user_id"       gorm:"index"` // 0 = unauthenticated
	IPAddress    string `json:"ip_address"    gorm:"size:64"`
}

func (RequestLog) TableName() string { return "srg_request_logs" }
