package model

type BotDetectionRule struct {
	CommonModel
	Pattern     string `json:"pattern"      gorm:"size:255;not null"`
	MatchField  string `json:"match_field"  gorm:"size:50;not null"` // "user_agent" | "absent_header"
	Type        string `json:"type"         gorm:"size:10;not null"` // "block" | "allow"
	Description string `json:"description"  gorm:"size:255"`
	IsEnabled   bool   `json:"is_enabled"   gorm:"default:true;not null"`
}

func (BotDetectionRule) TableName() string { return "srg_bot_detection_rules" }
