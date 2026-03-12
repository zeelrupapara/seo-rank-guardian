package model

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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
	Keywords     []JobKeyword   `json:"keywords" gorm:"foreignKey:JobID"`
	Regions      datatypes.JSON `json:"regions" swaggertype:"string"`
}

func (Job) TableName() string { return "srg_jobs" }

// NormalizeDomain strips protocol, www., and trailing slash from a domain string.
func NormalizeDomain(domain string) string {
	d := strings.TrimSpace(domain)
	d = strings.ToLower(d)
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "www.")
	d = strings.TrimRight(d, "/")
	return d
}

// ValidateDomain checks if a domain string looks valid after normalization.
func ValidateDomain(domain string) error {
	d := NormalizeDomain(domain)
	if d == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if strings.Contains(d, " ") {
		return fmt.Errorf("domain cannot contain spaces")
	}
	if !strings.Contains(d, ".") {
		return fmt.Errorf("domain must have a TLD (e.g. example.com)")
	}
	if strings.Contains(d, "/") {
		return fmt.Errorf("domain must not contain a path")
	}
	return nil
}

// FaviconURL returns the Google favicon API URL for a domain.
func FaviconURL(domain string) string {
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(domain) + "&sz=32"
}

func (j *Job) GetCompetitors() []string {
	var competitors []string
	if j.Competitors != nil {
		_ = json.Unmarshal(j.Competitors, &competitors)
	}
	return competitors
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

func (j *Job) SetRegions(regions []JobRegion) error {
	data, err := json.Marshal(regions)
	if err != nil {
		return err
	}
	j.Regions = data
	return nil
}
