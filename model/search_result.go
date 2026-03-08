package model

type SearchResult struct {
	CommonModel
	PairID       uint   `json:"pair_id" gorm:"index;not null"`
	RunID        uint   `json:"run_id" gorm:"index;not null"`
	JobID        uint   `json:"job_id" gorm:"index"`
	Domain       string `json:"domain" gorm:"size:255;not null"`
	Position     int    `json:"position"`
	URL          string `json:"url" gorm:"size:2048"`
	Title        string `json:"title" gorm:"size:500"`
	Snippet      string `json:"snippet" gorm:"type:text"`
	IsTarget     bool   `json:"is_target" gorm:"default:false"`
	IsCompetitor bool   `json:"is_competitor" gorm:"default:false"`
	Keyword      string `json:"keyword" gorm:"size:500"`
	State        string `json:"state" gorm:"size:255"`
}

func (SearchResult) TableName() string { return "srg_search_results" }
