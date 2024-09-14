package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/app/agent/repository"
	"github.com/DODOEX/web3rpcproxy/internal/app/shared"
	rdbscripts "github.com/DODOEX/web3rpcproxy/internal/app/shared/redis_scripts"
	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/allegro/bigcache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const CacheExpireInSeconds = time.Duration(7*24) * time.Hour

type TenantService interface {
	Access(ctx context.Context, token, bucket string) (*common.App, error)
	Affected(app *common.App) error
	Unaffected(app *common.App) error
}

// TenantService
type tenantService struct {
	logger           zerolog.Logger
	config           *config.Conf
	redis            *shared.RedisClient
	scripts          shared.Scripts
	tenantRepository repository.ITenantRepository
	rwm              sync.Map
	timers           sync.Map
	cache            *bigcache.BigCache
}

// init TenantService
func NewTenantService(config *config.Conf, logger zerolog.Logger, redis *shared.RedisClient, scripts shared.Scripts, tenantRepository repository.ITenantRepository) TenantService {
	_cacheConfig := bigcache.Config{
		// number of shards (must be a power of 2)
		Shards: 8,

		// time after which entry can be evicted
		LifeWindow: 1 * time.Hour,

		// Interval between removing expired entries (clean up).
		// If set to <= 0 then no action is performed.
		// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
		CleanWindow: 10 * time.Minute,

		// rps * lifeWindow, used only in initial memory allocation
		// MaxEntriesInWindow: 1000 * 10 * 60,

		// max entry size in bytes, used only in initial memory allocation
		// MaxEntrySize: 500,

		// prints information about additional memory allocation
		// Verbose: true,

		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 64,
	}
	config.Unmarshal("tenant.bigcache", &_cacheConfig)
	cache, initErr := bigcache.NewBigCache(_cacheConfig)
	if initErr != nil {
		log.Fatal(initErr)
	}

	service := &tenantService{
		config:           config,
		logger:           logger.With().Str("name", "tenant_service").Logger(),
		redis:            redis,
		scripts:          scripts,
		tenantRepository: tenantRepository,
		cache:            cache,
	}

	return service
}

func _TenantKey(args ...string) string {
	return helpers.Concat("app#", strings.Join(args, ":"))
}

func (s *tenantService) cacheTenantInfo(ctx context.Context, info *common.TenantInfo, expire time.Duration) error {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error().Interface("error", err).Msg("Failed to save tenant info to redis")
		}
	}()

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	s.redis.Client.Set(ctx, _TenantKey(info.Token), data, expire)
	return nil
}

func (s *tenantService) getTenantInfo(ctx context.Context, token string) (*common.TenantInfo, error) {
	logger := s.logger.Warn().Str("token", token)
	key := _TenantKey(token)

	l, _ := s.rwm.LoadOrStore(key, &sync.RWMutex{})
	rwm := l.(*sync.RWMutex)

	rwm.RLock()
	data, err := s.redis.Client.Get(ctx, key).Bytes()
	rwm.RUnlock()
	if err != nil && err != redis.Nil {
		logger.Err(err).Msg("Cache read error")
	}

	var info common.TenantInfo
	if len(data) > 0 {
		if err := json.Unmarshal(data, &info); err == nil {
			return &info, nil
		}
		logger.Msgf("Cache unmarshal error: %v", err)
	}

	s.logger.Debug().Str("token", token).Msg("Get app info with cache failed, try get from db")

	rwm.Lock()
	defer rwm.Unlock()
	if err := s.tenantRepository.GetTenantByToken(ctx, token, &info); err != nil {
		return nil, err
	}

	go s.cacheTenantInfo(context.Background(), &info, CacheExpireInSeconds)

	s.rwm.Delete(key)

	return &info, nil
}

func (s *tenantService) getBalanceValue(ctx context.Context, app *common.App) (int64, error) {
	var (
		capacity = int64(app.Capacity)
		rate     = int64(app.Rate)
	)
	balance, err := s.scripts.Balance(ctx, _TenantKey(app.Token, app.Bucket), capacity, rate)

	if err != nil {
		s.logger.Error().Str("token", app.Token).Str("bucket", app.Bucket).Int64("capacity", capacity).Int64("rate", rate).Msgf("Read balance error: %v", err)
		return 0, err
	}
	return balance, nil
}

func (s *tenantService) Access(ctx context.Context, token, bucket string) (*common.App, error) {
	key := _TenantKey(token, bucket)

	app := &common.App{}
	err := _GetCache(s.cache, key, app)
	if err != nil {
		info, err := s.getTenantInfo(ctx, token)

		if err != nil {
			return nil, err
		}

		app = &common.App{
			TenantInfo: *info,
			Bucket:     bucket,
		}
		_SetCache(s.cache, key, app)
	}

	balance, err := s.getBalanceValue(ctx, app)
	if err != nil {
		return nil, err
	}
	app.Balance = balance

	return app, nil
}

// 记录最后一次正确访问的时间，用于计算恢复量；异步调用时，只能保证最终准确性
func (s *tenantService) Affected(app *common.App) error {
	app.LastTime = time.Now().UnixMilli()

	if _, ok := s.timers.Load(_TenantKey(app.Token, app.Bucket)); !ok {
		go s.debounce(app)
	}

	return nil
}

// 记录异常访问的次数，用于补偿到balance；异步调用时，只能保证最终准确性
func (s *tenantService) Unaffected(app *common.App) error {
	atomic.AddInt64(&app.Offset, 1)

	if _, ok := s.timers.Load(_TenantKey(app.Token, app.Bucket)); !ok {
		go s.debounce(app)
	}

	return nil
}

// 流量防抖，每毫秒保存一次到redis
func (s *tenantService) debounce(app *common.App) error {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error().Interface("error", err).Msg("Panic to debounce() goroutine")
		}
	}()

	key := _TenantKey(app.Token, app.Bucket)
	timer := time.NewTimer(time.Millisecond)
	s.timers.Store(key, timer)
	defer s.timers.Delete(key)

	<-timer.C
	pipeline := s.redis.Client.Pipeline()

	if app.LastTime > 0 {
		pipeline.HSet(context.Background(), key, rdbscripts.CacheFieldLastTime, app.LastTime)
	}

	offset := atomic.LoadInt64(&app.Offset)
	if offset > 0 {
		pipeline.HIncrBy(context.Background(), key, rdbscripts.CacheFieldBalance, offset)
	}

	_, err := pipeline.Exec(context.Background())
	if err != nil {
		s.logger.Error().Err(err).Msgf("Failed to save redis with app")
		return err
	}

	if offset > 0 {
		atomic.AddInt64(&app.Offset, -offset)
		app.LastTime = 0
	}

	if v, err := json.Marshal(app); err == nil {
		err = s.cache.Set(key, v)
		if err != nil {
			s.logger.Error().Err(err).Msg("Cache update error")
		}
	}

	return nil
}
