package model

type IPBlockPolicy struct {
	CommonModel
	Name                 string `json:"name"                   gorm:"size:100;not null"`
	IsEnabled            bool   `json:"is_enabled"             gorm:"default:true;not null"`
	Description          string `json:"description"            gorm:"size:255"`
	// Level 1: accumulate this many rate-limit violations before the first block is applied
	L1ViolationThreshold int    `json:"l1_violation_threshold" gorm:"not null;default:10"`
	L1BlockSeconds       int    `json:"l1_block_seconds"       gorm:"not null;default:300"`     // 5 min
	// Level 2: escalate to L2 after this many L1 blocks
	L2ThresholdBlocks    int    `json:"l2_threshold_blocks"    gorm:"not null;default:5"`
	L2BlockSeconds       int    `json:"l2_block_seconds"       gorm:"not null;default:600"`     // 10 min
	// Level 3: escalate to L3 after this many L2 blocks
	L3ThresholdBlocks    int    `json:"l3_threshold_blocks"    gorm:"not null;default:3"`
	L3BlockSeconds       int    `json:"l3_block_seconds"       gorm:"not null;default:2592000"` // 30 days
}

func (IPBlockPolicy) TableName() string { return "srg_ip_block_policies" }
