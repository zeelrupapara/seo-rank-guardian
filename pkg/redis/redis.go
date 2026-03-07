package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

func NewRedisClient(cfg config.RedisConfig, log *zap.SugaredLogger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Info("Connected to Redis")
	return client, nil
}
