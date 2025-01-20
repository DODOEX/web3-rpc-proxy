package shared

import (
	"context"

	rdbscripts "github.com/DODOEX/web3rpcproxy/internal/app/shared/redis_scripts"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Scripts interface {
	Init(ctx context.Context) error
	Balance(ctx context.Context, key string, capacity int64, rate int64) (int64, error)
}

type scripts struct {
	logger  zerolog.Logger
	rdb     *RedisClient
	balance *redis.Script
}

func NewRedisScripts(rdb *RedisClient, logger zerolog.Logger) Scripts {
	rs := &scripts{
		rdb:     rdb,
		logger:  logger,
		balance: rdbscripts.GetBalanceScript(),
	}
	return rs
}

func (s *scripts) Init(ctx context.Context) error {
	result := s.balance.Load(context.Background(), s.rdb.Client)
	s.logger.Debug().Msgf("Redis scripts: balance=%s", s.balance.Hash())
	return result.Err()
}

func (s *scripts) Balance(ctx context.Context, key string, capacity int64, rate int64) (int64, error) {
	balance, err := s.balance.Run(ctx, s.rdb.Client, []string{key}, capacity, rate).Int64()
	return balance, err
}
