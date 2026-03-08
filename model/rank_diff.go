package model

type RankDiff struct {
	CommonModel
	JobID        uint   `json:"job_id" gorm:"index;not null"`
	RunID        uint   `json:"run_id" gorm:"index;not null"`
	PrevRunID    uint   `json:"prev_run_id"`
	Domain       string `json:"domain" gorm:"size:255"`
	PrevPosition int    `json:"prev_position"`
	CurrPosition int    `json:"curr_position"`
	Delta        int    `json:"delta"`
	ChangeType   string `json:"change_type" gorm:"size:50"`
	Keyword      string `json:"keyword" gorm:"size:500"`
	State        string `json:"state" gorm:"size:255"`
}

func (RankDiff) TableName() string { return "srg_rank_diffs" }
