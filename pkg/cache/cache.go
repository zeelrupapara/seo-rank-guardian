package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// luaIncrExpire atomically increments a counter and sets its expiry on first creation.
var luaIncrExpire = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return n
`)

type Cache struct {
	client *redis.Client
}

func NewCache(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// IncrWithExpiry atomically increments key and sets expiry on the first increment.
// Returns the new counter value. Safe to use for sliding-window rate limiting.
func (c *Cache) IncrWithExpiry(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return luaIncrExpire.Run(ctx, c.client, []string{key}, int(expiry.Seconds())).Int64()
}
