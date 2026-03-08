package model

import (
	"encoding/json"

	"gorm.io/datatypes"
)

type JobRegion struct {
	Country string `json:"country"`
	State   string `json:"state"`
	City    string `json:"city,omitempty"`
}

type Job struct {
	CommonModel
	UserID       uint           `json:"user_id" gorm:"index;not null"`
	Name         string         `json:"name" gorm:"size:255;not null"`
	Domain       string         `json:"domain" gorm:"size:255;not null"`
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	ScheduleTime string         `json:"schedule_time" gorm:"size:10"`
	Competitors  datatypes.JSON `json:"competitors" swaggertype:"array,string"`
	Keywords     datatypes.JSON `json:"keywords" swaggertype:"array,string"`
	Regions      datatypes.JSON `json:"regions" swaggertype:"string"`
}

func (Job) TableName() string { return "srg_jobs" }

func (j *Job) GetCompetitors() []string {
	var competitors []string
	if j.Competitors != nil {
		_ = json.Unmarshal(j.Competitors, &competitors)
	}
	return competitors
}

func (j *Job) GetKeywords() []string {
	var keywords []string
	if j.Keywords != nil {
		_ = json.Unmarshal(j.Keywords, &keywords)
	}
	return keywords
}

func (j *Job) GetRegions() []JobRegion {
	var regions []JobRegion
	if j.Regions != nil {
		_ = json.Unmarshal(j.Regions, &regions)
	}
	return regions
}

func (j *Job) SetCompetitors(competitors []string) error {
	data, err := json.Marshal(competitors)
	if err != nil {
		return err
	}
	j.Competitors = data
	return nil
}

func (j *Job) SetKeywords(keywords []string) error {
	data, err := json.Marshal(keywords)
	if err != nil {
		return err
	}
	j.Keywords = data
	return nil
}

func (j *Job) SetRegions(regions []JobRegion) error {
	data, err := json.Marshal(regions)
	if err != nil {
		return err
	}
	j.Regions = data
	return nil
}
