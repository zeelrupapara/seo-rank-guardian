package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/authz"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Middleware struct {
	App    *fiber.App
	DB     *gorm.DB
	Authz  *authz.Authz
	OAuth2 *oauth2.OAuth2
	Log    *zap.SugaredLogger
}

func NewMiddleware(app *fiber.App, db *gorm.DB, az *authz.Authz, o *oauth2.OAuth2, log *zap.SugaredLogger) *Middleware {
	return &Middleware{
		App:    app,
		DB:     db,
		Authz:  az,
		OAuth2: o,
		Log:    log,
	}
}
