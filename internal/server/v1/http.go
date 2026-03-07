package v1

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/internal/middleware"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/cache"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type HttpServer struct {
	App         *fiber.App
	Middleware  *middleware.Middleware
	DB          *gorm.DB
	Cache       *cache.Cache
	Log         *zap.SugaredLogger
	Validate    *validator.Validate
	Cfg         *config.Config
	OAuth2      *oauth2.OAuth2
	Nats        *natspkg.NatsClient
	GoogleOAuth *oauth2.GoogleOAuth
}

func NewHttpServer(
	app *fiber.App,
	mw *middleware.Middleware,
	db *gorm.DB,
	cache *cache.Cache,
	log *zap.SugaredLogger,
	validate *validator.Validate,
	cfg *config.Config,
	o *oauth2.OAuth2,
	nats *natspkg.NatsClient,
	googleOAuth *oauth2.GoogleOAuth,
) *HttpServer {
	return &HttpServer{
		App:         app,
		Middleware:  mw,
		DB:          db,
		Cache:       cache,
		Log:         log,
		Validate:    validate,
		Cfg:         cfg,
		OAuth2:      o,
		Nats:        nats,
		GoogleOAuth: googleOAuth,
	}
}
