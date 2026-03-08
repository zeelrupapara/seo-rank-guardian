package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/ai"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/db"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/logger"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/scraper"
	"github.com/zeelrupapara/seo-rank-guardian/worker"
)

func StartWorker() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log := logger.NewLogger(cfg.Logger)
	defer log.Sync()

	log.Info("Starting SEO Rank Guardian Worker...")

	pgDB, err := db.NewPostgresDB(cfg.Postgres, log)
	if err != nil {
		return err
	}

	natsClient, err := natspkg.NewNatsClient(cfg.NATS, log)
	if err != nil {
		return err
	}

	rodSearcher := scraper.NewRodSearcher(cfg.Scraper, log)
	collySearcher := scraper.NewCollySearcher(cfg.Scraper, log)
	chain := scraper.NewChainScraper(log, rodSearcher, collySearcher)

	aiClient, err := ai.NewAIClient(ai.AIClientConfig{
		Provider:        cfg.AI.Provider,
		APIKey:          cfg.AI.APIKey,
		Model:           cfg.AI.Model,
		SearchGrounding: cfg.AI.SearchGrounding,
	})
	if err != nil {
		log.Warnf("AI client init failed (reports will fail): %v", err)
		aiClient = nil
	}

	w := worker.NewWorker(natsClient, pgDB.DB, log, chain, aiClient, cfg)

	if err := w.Listen(); err != nil {
		return err
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("Shutting down worker...")
	w.Stop()
	return nil
}
