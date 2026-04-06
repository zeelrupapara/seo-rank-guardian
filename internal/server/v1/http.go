package v1

import (
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/internal/middleware"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/cache"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/manager"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	schedulerpkg "github.com/zeelrupapara/seo-rank-guardian/pkg/scheduler"
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
	Hub         *manager.Hub
	Scheduler   *schedulerpkg.Scheduler
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
	hub *manager.Hub,
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
		Hub:         hub,
	}
}

func parsePagination(c *fiber.Ctx) (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page", "1"))
	pageSize, _ = strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}
