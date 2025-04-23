package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/DODOEX/web3rpcproxy/internal/app/shared"
	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core"
	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/bytedance/sonic"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/rs/zerolog"
)

type agentServiceConfig struct {
	DisableCache bool
}

// AgentService
type agentService struct {
	logger     zerolog.Logger
	client     core.Client
	es         endpoint.Selector
	jrpcSchema *rpc.JSONRPCSchema
	cache      *shared.GlobalCache
	config     *agentServiceConfig
}

// define interface of IAgentService
//
//go:generate mockgen -destination=agent_service_mock.go -package=service . AgentService
type AgentService interface {
	Call(ctx context.Context, reqctx reqctx.Reqctxs, adapter endpoint.ChainAdapter, endpoints []*endpoint.Endpoint) ([]byte, error)
}

// init AgentService
func NewAgentService(
	logger zerolog.Logger,
	config *config.Conf,
	jrpcSchema *rpc.JSONRPCSchema,
	client core.Client,
	endpointService EndpointService,
	cache *shared.GlobalCache,
) AgentService {
	logger = logger.With().Str("name", "agent_service").Logger()

	existExpiryConfig := config.Exists("cache.results.expiry_durations")
	_config := &agentServiceConfig{
		DisableCache: config.Bool("cache.results.disable", false) || !existExpiryConfig,
	}

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

func (a agentService) Call(ctx context.Context, rc reqctx.Reqctxs, adapter endpoint.ChainAdapter, endpoints []*endpoint.Endpoint) ([]byte, error) {
	// 1. Unmarshal jsonrpcs
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
		if err = adapter.ValidateRequest(jsonrpcs[i]); err != nil {
			return nil, common.BadRequestError(err.Error(), err)
		}
	}

	// return one result if not batch call or batch call but only one result and it's error result
	isReturnOneResult := func(_results []rpc.JSONRPCResulter) bool {
		return !isBatchCall || (len(_results) == 1 && _results[0].Error() != nil)
	}

	// 2. Request directly if disable cache or cache is disabled
	if a.config.DisableCache || !rc.Options().Caches() {
		results, _err := a.call(ctx, rc, adapter, endpoints, jsonrpcs)
		if _err != nil {
			return nil, _err
		}

		if isReturnOneResult(results) {
			return sonic.Marshal(results[0])
		}

		return sonic.Marshal(results)
	}

	var (
		chainId           = rc.Chain().ID
		mapping           = map[string][]int{}
		notCachedJSONRPCs = []rpc.JSONRPCer{}
		results           = make([]rpc.JSONRPCResulter, len(jsonrpcs))
	)

	// 3. Read cache
	for i := 0; i < len(jsonrpcs); i++ {
		var data any
		// read cache
		if yes, key, _ := adapter.WithCache(chainId, jsonrpcs[i]); yes {
			// read mermory cache first
			if found, value, _ := adapter.GetEndpointValue(jsonrpcs[i], endpoints); found && value != nil {
				data = value
			} else if value, found, _ := a.cache.Get(key); found && value != nil {
				if err = sonic.Unmarshal(value, &data); err != nil {
					rc.Logger().Warn().Err(err).Msgf("Failed to unmarshal cache %s", jsonrpcs[i].Method())
				}
			}
		}

		var (
			appName = "unknown"
			method  = strings.Clone(jsonrpcs[i].Method())
		)
		if rc.App() != nil {
			appName = rc.App().Name
		}

		if data != nil {
			// hit, make result
			results[i] = jsonrpcs[i].MakeResult(data, nil)
			utils.TotalCaches.WithLabelValues(fmt.Sprint(chainId), appName, method, "mem").Inc()
		} else {
			// miss, add to _jsonrpcs
			notCachedJSONRPCs = append(notCachedJSONRPCs, jsonrpcs[i])
			utils.TotalCaches.WithLabelValues(fmt.Sprint(chainId), appName, method, "miss").Inc()

			id := jsonrpcs[i].ID()
			if mapping[id] == nil {
				mapping[id] = []int{}
			}
			mapping[id] = append(mapping[id], i)
		}
	}

	// return cache results
	if len(notCachedJSONRPCs) <= 0 {
		if isBatchCall {
			return sonic.Marshal(results)
		} else if len(results) > 0 {
			return results[0].MarshalJSON()
		}
	}

	// 4. Request not cached jsonrpcs
	_results, err := a.call(ctx, rc, adapter, endpoints, notCachedJSONRPCs)
	if err != nil {
		return nil, err
	}

	if isReturnOneResult(results) {
		return sonic.Marshal(_results[0])
	}

	// assembly results
	for i := range _results {
		indexes := mapping[_results[i].ID()]

		for _, index := range indexes {
			// skip if result is already set
			if results[index] != nil {
				continue
			}
			results[index] = _results[i]
		}
	}
	return sonic.Marshal(results)
}

func (a agentService) call(ctx context.Context, rc reqctx.Reqctxs, adapter endpoint.ChainAdapter, endpoints []*endpoint.Endpoint, jsonrpcs []rpc.JSONRPCer) (results []rpc.JSONRPCResulter, err error) {
	chainId := rc.Chain().ID
	// select available endpoints
	_endpoints, ok := adapter.Select(ctx, rc, endpoints, jsonrpcs)
	if !ok || len(_endpoints) <= 0 {
		a.logger.Error().Msgf("%d No available endpoints", chainId)
		return nil, common.InternalServerError("No available endpoints")
	}

	// rebuild jsonrpc id
	var (
		prefix    = helpers.Short(rc.ReqID())
		_jsonrpcs = make([]rpc.JSONRPCer, len(jsonrpcs))
	)
	for i := range jsonrpcs {
		_jsonrpcs[i] = jsonrpcs[i].Clone(map[string]any{
			"id": prefix + jsonrpcs[i].ID(),
		}).(rpc.JSONRPCer)
	}

	// request to endpoints
	_results, err := a.client.Request(ctx, rc, adapter, _endpoints, _jsonrpcs)

	if err != nil {
		return nil, err
	}
	// return directly if no results
	if len(_results) <= 0 || (len(_results) == 1 && _results[0].Error() != nil) {
		return _results, nil
	}

	// revert jsonrpc id
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

	// save cache
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

			// get cache options
			yes, key, _ := adapter.WithCache(chainId, *jsonrpc)
			if !yes {
				continue
			}

			data, err := sonic.Marshal(results[i].Result())
			if err != nil {
				continue
			}

			a.cache.Set(key, data)
			// a.logger.Debug().Msgf("Cache capacity: %d, len: %d", a.cache.Capacity(), a.cache.Len())
		}
	}

	return results, nil
}
