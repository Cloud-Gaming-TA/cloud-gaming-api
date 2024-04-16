package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Context context.Context
	Client  *redis.Client
}

func NewRedisClient(options *redis.Options) *RedisClient {
	ctx := context.Background()

	return &RedisClient{
		Client:  redis.NewClient(options),
		Context: ctx,
	}
}
