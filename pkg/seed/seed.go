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
		// user role — app access
		{"user", authz.ResourceProfile, authz.ActionRead},
		{"user", authz.ResourceProfile, authz.ActionWrite},
		{"user", authz.ResourceDashboard, authz.ActionRead},
		{"user", authz.ResourceJobs, authz.ActionRead},
		{"user", authz.ResourceJobs, authz.ActionWrite},
		{"user", authz.ResourceJobs, authz.ActionDelete},
		{"user", authz.ResourceRuns, authz.ActionRead},
		{"user", authz.ResourceReports, authz.ActionRead},

		// admin role — app access (same as user)
		{"admin", authz.ResourceProfile, authz.ActionRead},
		{"admin", authz.ResourceProfile, authz.ActionWrite},
		{"admin", authz.ResourceDashboard, authz.ActionRead},
		{"admin", authz.ResourceJobs, authz.ActionRead},
		{"admin", authz.ResourceJobs, authz.ActionWrite},
		{"admin", authz.ResourceJobs, authz.ActionDelete},
		{"admin", authz.ResourceRuns, authz.ActionRead},
		{"admin", authz.ResourceReports, authz.ActionRead},

		// admin role — admin-only resources
		{"admin", authz.ResourceUsers, authz.ActionRead},
		{"admin", authz.ResourceUsers, authz.ActionWrite},
		{"admin", authz.ResourceUsers, authz.ActionDelete},
		{"admin", authz.ResourcePolicies, authz.ActionRead},
		{"admin", authz.ResourcePolicies, authz.ActionWrite},
		{"admin", authz.ResourcePolicies, authz.ActionDelete},
		{"admin", authz.ResourceSessions, authz.ActionRead},
		{"admin", authz.ResourceSessions, authz.ActionDelete},
		{"admin", authz.ResourceAudit, authz.ActionRead},
		{"admin", authz.ResourceIPFilters, authz.ActionRead},
		{"admin", authz.ResourceIPFilters, authz.ActionWrite},
		{"admin", authz.ResourceIPFilters, authz.ActionDelete},
		{"admin", authz.ResourceRateLimits, authz.ActionRead},
		{"admin", authz.ResourceRateLimits, authz.ActionWrite},
		{"admin", authz.ResourceRateLimits, authz.ActionDelete},

		// bot_detection resource
		{"admin", authz.ResourceBotDetection, authz.ActionRead},
		{"admin", authz.ResourceBotDetection, authz.ActionWrite},
		{"admin", authz.ResourceBotDetection, authz.ActionDelete},

		// analytics resource
		{"admin", authz.ResourceAnalytics, authz.ActionRead},

		// ip_block_policies resource
		{"admin", authz.ResourceIPBlockPolicies, authz.ActionRead},
		{"admin", authz.ResourceIPBlockPolicies, authz.ActionWrite},
		{"admin", authz.ResourceIPBlockPolicies, authz.ActionDelete},

		// auto_ip_blocks resource
		{"admin", authz.ResourceAutoIPBlocks, authz.ActionRead},
		{"admin", authz.ResourceAutoIPBlocks, authz.ActionWrite},
		{"admin", authz.ResourceAutoIPBlocks, authz.ActionDelete},
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
