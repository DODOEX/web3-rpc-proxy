package shared

import (
	"context"
	"time"

	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type RedisClient struct {
	logger zerolog.Logger
	config *config.Conf
	Client *redis.Client
}

func NewRedisClient(config *config.Conf, logger zerolog.Logger) *RedisClient {
	return &RedisClient{
		logger: logger,
		Client: nil,
		config: config,
	}
}

func (r *RedisClient) Connect(ctx context.Context) error {
	opts, err := redis.ParseURL(r.config.String("redis.url"))
	if err != nil {
		return err
	}

	r.Client = redis.NewClient(opts)
	if err := r.Client.Ping(ctx).Err(); err != nil {
		return err
	}

	return nil
}

func (r *RedisClient) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Close()
}

func (r *RedisClient) reconnect() {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Error().Interface("error", err).Msg("Failed to reconnect to redis")
		}
		time.Sleep(r.config.Duration("redis.keeplive-interval"))
		r.reconnect()
	}()

	opts, err := redis.ParseURL(r.config.String("redis.url"))
	if err != nil {
		r.logger.Panic().Err(err)
	}

	retryCount := r.config.Int("redis.retry-count")
	for {
		if r.Client == nil {
			for i := 1; i <= retryCount; i++ {
				_, err := r.Client.Ping(context.Background()).Result()
				if err == nil || err == redis.Nil {
					r.logger.Info().Msgf("Reconnected to Redis succesfully!")
					break
				} else {
					if i == retryCount {
						r.Close()
						r.logger.Error().Msgf("Failed to connect to Redis: %v. Retryed %v times\n", err, i)
						return
					}

					r.logger.Warn().Msgf("Failed to connect to Redis: %v. Retrying in %v...\n", err, i)
					r.Client = redis.NewClient(opts)
				}
			}
		} else {
			r.Client = redis.NewClient(opts)
		}

	}
}
