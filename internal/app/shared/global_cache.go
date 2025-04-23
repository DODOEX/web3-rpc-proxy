package shared

import (
	"log"
	"time"

	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/allegro/bigcache"
	"github.com/bytedance/sonic"
	"github.com/rs/zerolog"
)

type CacheEntry struct {
	V          any
	E          int64
	Compressed bool
}

type GlobalCacheConfig struct {
	CacheMethods      map[string]string
	MaxEntryCacheSize int
}

type GlobalCache struct {
	logger zerolog.Logger
	config *GlobalCacheConfig
	cache  *bigcache.BigCache
}

func NewGlobalCache(
	logger zerolog.Logger,
	config *config.Conf,
) *GlobalCache {
	_logger := logger.With().Str("name", "global_cache").Logger()

	existExpiryConfig := config.Exists("cache.results.expiry_durations")
	_config := &GlobalCacheConfig{
		MaxEntryCacheSize: 512 * 1024, // 512KB
	}

	if existExpiryConfig {
		expiryConfig := map[string]string{}
		config.Unmarshal("cache.results.expiry_durations", &expiryConfig)
		_config.CacheMethods = expiryConfig
	}

	// default cache size is 512MB
	totalCacheSize := config.Int("cache.results.size", 512*1024*1024)

	endpointLen := 16
	_endpoints := config.Get(KoanfEndpointsToken, []any{})
	if v, ok := _endpoints.([]interface{}); ok {
		endpointLen = len(v)
	}
	shards := int(nearestPowerOfTwo(uint(endpointLen)))
	// must have 8MB size pre shard
	if totalCacheSize/shards < 8 {
		shards = int(nearestPowerOfTwo(uint(totalCacheSize / 8)))
	}

	_cacheConfig := bigcache.Config{
		// number of shards (must be a power of 2)
		Shards: shards,

		// time after which entry can be evicted
		LifeWindow: 15 * time.Minute,

		// Interval between removing expired entries (clean up).
		// If set to <= 0 then no action is p4erformed.
		// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
		CleanWindow: 15 * time.Minute,

		// rps * lifeWindow, used only in initial memory allocation
		// MaxEntriesInWindow: 1000 * 10 * 60,

		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: _config.MaxEntryCacheSize,

		// prints information about additional memory allocation
		// Verbose: true,

		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: totalCacheSize / 1024 / 1024,
	}
	config.Unmarshal("agent.bigcache", &_cacheConfig)

	cache, initErr := bigcache.NewBigCache(_cacheConfig)
	if initErr != nil {
		log.Fatal(initErr)
	}

	logger.Info().Msgf("Cache size: %d MB", _cacheConfig.HardMaxCacheSize)
	return &GlobalCache{
		logger: _logger,
		config: _config,
		cache:  cache,
	}
}

func nearestPowerOfTwo(n uint) uint {
	if n == 0 {
		return 1
	}

	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++

	return n
}

// write to cache
func (c *GlobalCache) set(k string, v CacheEntry) error {
	if data, err := sonic.Marshal(v); err == nil {
		err = c.cache.Set(k, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *GlobalCache) Set(key string, value []byte, ttls ...time.Duration) error {
	var expiredAt int64 = 0
	if len(ttls) > 0 {
		expiredAt = int64(time.Now().Add(ttls[0]).UnixMilli())
	}

	if len(value) > c.config.MaxEntryCacheSize {
		// compress data and write to cache in a goroutine
		// if compress failed, write data to cache directly

		go func(k string, v []byte) {
			defer func() {
				if err := recover(); err != nil {
					c.logger.Error().Interface("error", err).Msg("Failed to set cache result")
				}
			}()

			if compressed, err := helpers.Compress(v); err != nil {
				c.logger.Error().Err(err).Msg("Failed to compress")
			} else {
				// skip set cache, data is bigger than cache size after compression
				if len(compressed) > c.config.MaxEntryCacheSize {
					return
				}
				v = compressed
			}

			if err := c.set(k, CacheEntry{V: v, E: expiredAt, Compressed: true}); err != nil {
				c.logger.Error().Err(err).Msg("Cache set error")
			}
		}(key, value)
	} else {
		// write to cache
		if err := c.set(key, CacheEntry{V: value, E: expiredAt}); err != nil {
			return err
		}
	}
	return nil
}

func (c *GlobalCache) get(k string) (CacheEntry, error) {
	v := CacheEntry{}
	data, err := c.cache.Get(k)
	if err == nil {
		err = sonic.Unmarshal(data, v)
	}
	return v, err
}

func (c *GlobalCache) Get(key string) (value []byte, found bool, err error) {
	entry, err := c.get(key)
	if err != nil {
		return nil, false, err
	}
	if time.UnixMilli(entry.E).After(time.Now()) {
		go c.cache.Delete(key)
		return nil, true, nil
	}
	// uncompress
	if entry.Compressed {
		_v, _err := helpers.Decompress(entry.V.([]byte))
		if _err != nil {
			return nil, true, _err
		}
		return _v, true, nil
	}
	return entry.V.([]byte), true, nil
}

func (c *GlobalCache) Del(k string) error {
	return c.cache.Delete(k)
}
