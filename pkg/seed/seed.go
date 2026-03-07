package seed

import (
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/authz"
	"github.com/zeelrupapara/seo-rank-guardian/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Run(db *gorm.DB, az *authz.Authz, log *zap.SugaredLogger) error {
	policies := [][]string{
		{"admin", authz.ResourceUsers, authz.ActionRead},
		{"admin", authz.ResourceUsers, authz.ActionWrite},
		{"admin", authz.ResourceUsers, authz.ActionDelete},
		{"admin", authz.ResourceProfile, authz.ActionRead},
		{"admin", authz.ResourceProfile, authz.ActionWrite},
		{"user", authz.ResourceProfile, authz.ActionRead},
		{"user", authz.ResourceProfile, authz.ActionWrite},
	}

	for _, p := range policies {
		if _, err := az.Enforcer.AddPolicy(p); err != nil {
			log.Warnf("Policy may already exist: %v", err)
		}
	}

	if err := az.Enforcer.SavePolicy(); err != nil {
		return err
	}

	log.Info("Casbin policies seeded")

	var count int64
	db.Model(&model.User{}).Where("role = ?", "admin").Count(&count)
	if count == 0 {
		hashedPw, err := utils.HashPassword("admin123")
		if err != nil {
			return err
		}

		admin := model.User{
			Username: "admin",
			Email:    "admin@srg.local",
			Password: hashedPw,
			Role:     "admin",
			IsActive: true,
		}

		if err := db.Create(&admin).Error; err != nil {
			return err
		}

		log.Info("Admin user seeded")
	}

	return nil
}
