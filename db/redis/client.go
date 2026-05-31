package redis

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

func NewRedisClient() *redis.Client {
	Client = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
		Protocol: 2,
	})

	return Client
}
