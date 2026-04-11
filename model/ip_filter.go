package model

type IPFilter struct {
	CommonModel
	IPRange     string `json:"ip_range"    gorm:"size:50;not null;uniqueIndex"`
	Type        string `json:"type"        gorm:"size:10;not null"`  // "allow" | "block"
	Description string `json:"description" gorm:"size:255"`
	IsEnabled   bool   `json:"is_enabled"  gorm:"default:true;not null"`
}

func (IPFilter) TableName() string { return "srg_ip_filters" }
