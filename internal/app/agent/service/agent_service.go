package service

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core"
	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/allegro/bigcache"
	"github.com/bytedance/sonic"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/rs/zerolog"
)

type CacheEntry struct {
	V          any
	T          int64
	compressed bool
}
type agentServiceConfig struct {
	CacheMethods      map[string]string
	MaxEntryCacheSize int
	DisableCache      bool
}

// AgentService
type agentService struct {
	logger     zerolog.Logger
	client     core.Client
	es         endpoint.Selector
	jrpcSchema *rpc.JSONRPCSchema
	cache      *bigcache.BigCache
	config     *agentServiceConfig
}

// define interface of IAgentService
//
//go:generate mockgen -destination=agent_service_mock.go -package=service . AgentService
type AgentService interface {
	Call(ctx context.Context, reqctx reqctx.Reqctxs, endpoints []*endpoint.Endpoint) ([]byte, error)
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

// init AgentService
func NewAgentService(
	logger zerolog.Logger,
	config *config.Conf,
	jrpcSchema *rpc.JSONRPCSchema,
	client core.Client,
	endpointService EndpointService,
) AgentService {
	logger = logger.With().Str("name", "agent_service").Logger()

	existExpiryConfig := config.Exists("cache.results.expiry_durations")
	_config := &agentServiceConfig{
		DisableCache:      config.Bool("cache.results.disable", false) || !existExpiryConfig,
		MaxEntryCacheSize: 512 * 1024, // 512KB
	}

	if existExpiryConfig {
		expiryConfig := map[string]string{}
		config.Unmarshal("cache.results.expiry_durations", &expiryConfig)
		_config.CacheMethods = expiryConfig
	}

	// default cache size is 512MB
	totalCacheSize := config.Int("cache.results.size", 512*1024*1024)

	shards := int(nearestPowerOfTwo(uint(len(endpointService.Chains()))))
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

	service := agentService{
		config:     _config,
		client:     client,
		logger:     logger,
		jrpcSchema: jrpcSchema,
		cache:      cache,
		es:         endpoint.NewSelector(),
	}

	return service
}

func (a agentService) Call(ctx context.Context, rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint) ([]byte, error) {
	// 1. 解析到jsonrpc数组
	jsonrpcs, isBatchCall, err := rpc.UnmarshalJSONRPCs(*rc.Body())
	if err != nil {
		return nil, common.BadRequestError(err.Error(), err)
	}
	if len(jsonrpcs) == 0 {
		if isBatchCall {
			return []byte("[{\"id\": null}]"), nil
		}
		return []byte("{\"id\": null}"), nil
	}

	for i := range jsonrpcs {
		if err = a.jrpcSchema.ValidateRequest(jsonrpcs[i].Method(), jsonrpcs[i].Map()); err != nil {
			return nil, common.BadRequestError(err.Error(), err)
		}
	}

	// 发出实际调用请求
	dispatch := func(data []rpc.JSONRPCer) (any, error) {
		if len(data) == 0 {
			if isBatchCall {
				return []rpc.JSONRPCResulter{}, nil
			}
			return rpc.NewJSONRPC(), nil
		}

		// 批量调用
		results, _err := a.call(ctx, rc, endpoints, data)

		if _err != nil {
			return nil, _err
		}

		// - 返回异常结果
		// - 返回单个调用的结果
		if !isBatchCall || (len(results) == 1 && results[0].Error() != nil) {
			return results[0], nil
		}

		// 返回批量调用的结果
		return results, nil
	}

	// 处理调用请求
	handle := func(_jsonrpcs []rpc.JSONRPCer) ([]byte, error) {
		results, _err := dispatch(_jsonrpcs)

		if _err != nil {
			return nil, _err
		}

		return sonic.Marshal(results)
	}

	// 2. 如果不使用缓存，则直接调用
	if a.config.DisableCache || !rc.Options().Caches() {
		return handle(jsonrpcs)
	}

	// 3. 从缓存中获取结果
	var (
		chainId   = rc.ChainID()
		mapping   = map[string][]int{}
		_jsonrpcs = []rpc.JSONRPCer{}
		results   = make([]rpc.JSONRPCResulter, len(jsonrpcs))
	)

	for i := 0; i < len(jsonrpcs); i++ {
		var v any
		// 读 cache
		if ok, ttl := _WithCache(a.config.CacheMethods, jsonrpcs[i]); ok {
			key, entry := _CacheKey(chainId, jsonrpcs[i]), &CacheEntry{}
			err = _GetCache(a.cache, key, entry)
			if err == nil {
				if time.UnixMilli(entry.T).Add(ttl).After(time.Now()) {
					// 解压
					if entry.compressed {
						if _v, _err := helpers.Decompress(entry.V.([]byte)); _err != nil {
							rc.Logger().Warn().Err(_err).Msgf("Failed to compress cache %s", jsonrpcs[i].Method())
						} else if err = sonic.Unmarshal(_v, &v); err != nil {
							rc.Logger().Warn().Err(err).Msgf("Failed to unmarshal cache %s", jsonrpcs[i].Method())
						}
					} else {
						v = entry.V
					}

					if jsonrpcs[i].Method() == "eth_blockNumber" {
						endpoint := slices.MaxFunc(endpoints, func(a *endpoint.Endpoint, b *endpoint.Endpoint) int {
							return int(b.BlockNumber() - a.BlockNumber())
						})
						if height := endpoint.BlockNumber(); height > 0 {
							if v == nil {
								v = height
							} else if n, _err := strconv.ParseUint(v.(string), 16, 64); _err == nil {
								v = slices.Max([]uint64{height, n})
							}
						}
					}
				} else {
					go a.cache.Delete(key)
				}
			}
		}

		appName := "unknown"
		if rc.App() != nil {
			appName = rc.App().Name
		}

		method := strings.Clone(jsonrpcs[i].Method())
		if v != nil {
			// hit, 组装结果
			results[i] = jsonrpcs[i].MakeResult(v, nil)
			utils.TotalCaches.WithLabelValues(fmt.Sprint(chainId), appName, method, "mem").Inc()
		} else {
			// miss, 组装新请求
			_jsonrpcs = append(_jsonrpcs, jsonrpcs[i])
			utils.TotalCaches.WithLabelValues(fmt.Sprint(chainId), appName, method, "miss").Inc()

			id := jsonrpcs[i].ID()
			if mapping[id] == nil {
				mapping[id] = []int{}
			}
			mapping[id] = append(mapping[id], i)
		}
	}

	// 直接返回缓存结果
	if len(_jsonrpcs) <= 0 {
		if isBatchCall {
			return sonic.Marshal(results)
		} else if len(results) > 0 {
			return results[0].MarshalJSON()
		}
	}

	// 发起节点请求
	data, err := dispatch(_jsonrpcs)

	if err != nil {
		return nil, err
	}

	// 将请求结果填充到最终结果中
	if _results, ok := data.([]rpc.JSONRPCResulter); ok {
		for i := range _results {
			indexes := mapping[_results[i].ID()]

			for _, index := range indexes {
				// 如果已经有缓存结果，则跳过
				if results[index] != nil {
					continue
				}
				results[index] = _results[i]
			}
		}

		return sonic.Marshal(results)
	}

	return sonic.Marshal(data)
}

func (a agentService) call(ctx context.Context, rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint, jsonrpcs []rpc.JSONRPCer) (results []rpc.JSONRPCResulter, err error) {
	chainId := rc.ChainID()
	// 获取_endpoints
	_endpoints, ok := a.es.Select(ctx, rc, endpoints, jsonrpcs)
	if !ok || len(_endpoints) <= 0 {
		a.logger.Error().Msgf("%d No available endpoints", chainId)
		return nil, common.InternalServerError("No available endpoints")
	}

	// 改写 ID
	var (
		prefix    = helpers.Short(rc.ReqID())
		_jsonrpcs = make([]rpc.JSONRPCer, len(jsonrpcs))
	)
	for i := range jsonrpcs {
		_jsonrpcs[i] = jsonrpcs[i].Clone(map[string]any{
			"id": prefix + jsonrpcs[i].ID(),
		}).(rpc.JSONRPCer)
	}

	// 请求节点（没有命中缓存的jsonrpc）
	_results, err := a.client.Request(ctx, rc, _endpoints, _jsonrpcs)

	if err != nil {
		return nil, err
	}
	// 空结果和异常结果直接返回
	if len(_results) <= 0 || (len(_results) == 1 && _results[0].Error() != nil) {
		return _results, nil
	}

	// 绑定 jsonrpc, 方便json.Marshal()时， jsonrpc, id 与jsonrpcs保持一致
	results = make([]rpc.JSONRPCResulter, len(_results))
	if len(jsonrpcs) == 1 && len(_results) == 1 {
		results[0] = jsonrpcs[0].MakeResult(_results[0].Result(), _results[0].Error())
	} else if len(_results) > 1 {
		for i := range _results {
			if j := slices.IndexFunc(_jsonrpcs, func(_jsonrpc rpc.JSONRPCer) bool {
				return _jsonrpc.ID() == _results[i].ID()
			}); j > -1 {
				results[i] = jsonrpcs[j].MakeResult(_results[i].Result(), _results[i].Error())
			}
		}
	}

	// 将结果批量写入缓存
	if !a.config.DisableCache {
		for i := range results {
			if results[i].Type() != rpc.JSONRPC_RESPONSE {
				continue
			}

			jsonrpc, ok := slice.Find(jsonrpcs, func(_ int, jsonrpc rpc.JSONRPCer) bool {
				return jsonrpc.ID() == results[i].ID()
			})
			if !ok {
				continue
			}

			// 根据配置，判断是否需要缓存
			ok, _ = _WithCache(a.config.CacheMethods, *jsonrpc)
			if !ok {
				continue
			}

			key := _CacheKey(chainId, *jsonrpc)
			data, err := sonic.Marshal(results[i].Result())
			if err != nil {
				continue
			}

			if len(data) > a.config.MaxEntryCacheSize {
				// 压缩
				go func(k string, v []byte) {
					defer func() {
						if err := recover(); err != nil {
							a.logger.Error().Interface("error", err).Msg("Failed to set cache result")
						}
					}()

					if compressed, err := helpers.Compress(v); err != nil {
						a.logger.Error().Err(err).Msg("Failed to compress")
					} else {
						// skip set cache, data is bigger than cache size after compression
						if len(compressed) > a.config.MaxEntryCacheSize {
							return
						}
						v = compressed
					}

					// 写内存
					if err := _SetCache(a.cache, k, &CacheEntry{V: v, T: time.Now().UnixMilli(), compressed: true}); err != nil {
						a.logger.Error().Err(err).Msg("Cache set error")
					}
				}(key, data)
			} else {
				if err := _SetCache(a.cache, key, &CacheEntry{V: results[i].Result(), T: time.Now().UnixMilli()}); err != nil {
					a.logger.Error().Err(err).Msg("Cache set error")
				}
			}

			a.logger.Debug().Msgf("Cache capacity: %d, len: %d", a.cache.Capacity(), a.cache.Len())
		}
	}

	return results, nil
}
