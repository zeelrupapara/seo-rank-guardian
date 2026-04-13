package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-playground/validator/v10"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/internal/middleware"
	"github.com/zeelrupapara/seo-rank-guardian/internal/server"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/authz"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/cache"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/db"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/logger"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/logrotate"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/manager"
	miniopkg "github.com/zeelrupapara/seo-rank-guardian/pkg/minio"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	pkgoauth2 "github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	redispkg "github.com/zeelrupapara/seo-rank-guardian/pkg/redis"
	schedulerpkg "github.com/zeelrupapara/seo-rank-guardian/pkg/scheduler"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/seed"
)

func Start() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log, rotatingWriter := logger.NewDynamicLogger(cfg.Logger)
	defer log.Sync()

	log.Info("Starting SEO Rank Guardian API server...")

	redisClient, err := redispkg.NewRedisClient(cfg.Redis, log)
	if err != nil {
		return err
	}

	appCache := cache.NewCache(redisClient)

	pgDB, err := db.NewPostgresDB(cfg.Postgres, log)
	if err != nil {
		return err
	}

	if err := pgDB.Migrate(); err != nil {
		return err
	}

	natsClient, err := natspkg.NewNatsClient(cfg.NATS, log)
	if err != nil {
		return err
	}

	if err := natsClient.EnsureStream("SRG_JOBS", []string{"srg.jobs.>"}); err != nil {
		return err
	}

	if err := natsClient.EnsureStream("SRG_LOGS", []string{"srg.logs.>"}); err != nil {
		return err
	}

	if err := natsClient.EnsureStream("SRG_WS", []string{"srg.ws.>"}); err != nil {
		return err
	}

	az, err := authz.NewAuthz(pgDB.DB, "pkg/authz/model.conf", log)
	if err != nil {
		return err
	}

	o, err := pkgoauth2.NewOAuth2(cfg.OAuth, redisClient, log)
	if err != nil {
		return err
	}

	var googleOAuth *pkgoauth2.GoogleOAuth
	if cfg.Google.ClientID != "" {
		googleOAuth = pkgoauth2.NewGoogleOAuth(cfg.Google, log)
		log.Info("Google OAuth initialized")
	}

	if err := seed.Run(pgDB.DB, az, log); err != nil {
		log.Warnf("Seed warning: %v", err)
	}

	validate := validator.New()

	hub := manager.NewHub(log)

	app := httputil.NewApp()
	mw := middleware.NewMiddleware(app, pgDB.DB, az, o, log)

	srv := server.NewServer(app, mw, pgDB.DB, appCache, log, validate, cfg, o, natsClient, googleOAuth, hub)
	srv.Register()

	// Init scheduler — wraps triggerScrapeForJob as a TriggerFunc
	sched := schedulerpkg.New(pgDB.DB, log, func(job model.Job, triggeredBy string, userID uint) error {
		_, err := srv.HttpServer.TriggerScrapeForJob(job, triggeredBy, userID)
		return err
	})
	if err := sched.LoadAll(); err != nil {
		log.Warnf("Scheduler load warning: %v", err)
	}
	srv.HttpServer.Scheduler = sched
	sched.Start()

	// MinIO client for log uploads (non-fatal if unavailable).
	var minioClient *miniopkg.Client
	mc, err := miniopkg.New(cfg.MinIO, log)
	if err != nil {
		log.Warnf("MinIO init warning (log uploads disabled): %v", err)
	} else {
		if err := mc.EnsureBucket(context.Background()); err != nil {
			log.Warnf("MinIO bucket init warning: %v", err)
		} else {
			minioClient = mc
		}
	}

	logRotator := logrotate.New(minioClient, rotatingWriter, cfg.Logger.Dir, log)
	logRotator.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf("%s:%s", cfg.HTTP.Host, cfg.HTTP.Port)
		log.Infof("Listening on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
		log.Info("Shutting down server...")
	sched.Stop()
	logRotator.Stop()
	natsClient.Close()
	return app.Shutdown()
}
