package model

type User struct {
	CommonModel
	Username   string `json:"username" gorm:"uniqueIndex;size:255"`
	Email      string `json:"email" gorm:"uniqueIndex;size:255"`
	Password   string `json:"-" gorm:"size:255"`
	Role       string `json:"role" gorm:"size:50;default:user"`
	IsActive   bool   `json:"is_active" gorm:"default:true"`
	Provider   string `json:"provider" gorm:"size:50;default:local"`
	ProviderID string `json:"provider_id,omitempty" gorm:"size:255"`
}

func (User) TableName() string {
	return "srg_users"
}
