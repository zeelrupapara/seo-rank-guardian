package db

import (
	"fmt"
	"time"

	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type PostgresDB struct {
	DB  *gorm.DB
	Log *zap.SugaredLogger
}

func NewPostgresDB(cfg config.PostgresConfig, log *zap.SugaredLogger) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DB, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: "srg_",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Info("Connected to PostgreSQL")

	return &PostgresDB{DB: db, Log: log}, nil
}

func (p *PostgresDB) Migrate() error {
	err := p.DB.AutoMigrate(
		&model.User{},
		&model.Job{},
		&model.JobKeyword{},
		&model.JobRun{},
		&model.SearchPair{},
		&model.SearchResult{},
		&model.RankDiff{},
		&model.Report{},
		&model.RunEventLog{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}
	p.Log.Info("Database migration completed")
	return nil
}
