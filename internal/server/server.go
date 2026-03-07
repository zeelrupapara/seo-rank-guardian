package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/internal/middleware"
	v1 "github.com/zeelrupapara/seo-rank-guardian/internal/server/v1"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/cache"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/go-playground/validator/v10"
)

type Server struct {
	App        *fiber.App
	Middleware *middleware.Middleware
	HttpServer *v1.HttpServer
	Log        *zap.SugaredLogger
	Cfg        *config.Config
	Nats       *natspkg.NatsClient
}

func NewServer(
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
) *Server {
	httpServer := v1.NewHttpServer(app, mw, db, cache, log, validate, cfg, o, nats, googleOAuth)

	return &Server{
		App:        app,
		Middleware: mw,
		HttpServer: httpServer,
		Log:        log,
		Cfg:        cfg,
		Nats:       nats,
	}
}
