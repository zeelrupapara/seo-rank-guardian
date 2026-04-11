package model

type RateLimit struct {
	CommonModel
	Role          string `json:"role"           gorm:"size:50;not null;index"`
	Endpoint      string `json:"endpoint"       gorm:"size:200;not null"` // "*" or Fiber route pattern e.g. "/api/v1/jobs/:jobId/scrape"
	MaxRequests   int    `json:"max_requests"   gorm:"not null"`
	WindowSeconds int    `json:"window_seconds" gorm:"not null"`
	IsEnabled     bool   `json:"is_enabled"     gorm:"default:true;not null"`
	Description   string `json:"description"    gorm:"size:255"`
}

func (RateLimit) TableName() string { return "srg_rate_limits" }
