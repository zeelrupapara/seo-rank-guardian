package model

import (
	"fmt"
	"net/url"
)

type User struct {
	CommonModel
	Username   string `json:"username" gorm:"uniqueIndex;size:255"`
	Email      string `json:"email" gorm:"uniqueIndex;size:255"`
	Password   string `json:"-" gorm:"size:255"`
	Role       string `json:"role" gorm:"size:50;default:user"`
	IsActive   bool   `json:"is_active" gorm:"default:true"`
	Provider   string `json:"provider" gorm:"size:50;default:local"`
	ProviderID string `json:"provider_id,omitempty" gorm:"size:255"`
	AvatarURL  string `json:"avatar_url" gorm:"size:500"`
}

func (User) TableName() string {
	return "srg_users"
}

// DefaultAvatarURL generates a free avatar URL from ui-avatars.com based on the given name.
func DefaultAvatarURL(name string) string {
	return fmt.Sprintf("https://ui-avatars.com/api/?name=%s&size=128&background=random&bold=true", url.QueryEscape(name))
}
