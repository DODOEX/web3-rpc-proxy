package adapters

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/bytedance/sonic"
	"github.com/duke-git/lancet/v2/slice"
)

func NewEvmAdapter(
	config *config.Conf,
	jrpcSchema *rpc.JSONRPCSchema,
	selector endpoint.Selector,
) endpoint.ChainAdapter {
	conf := map[string]string{}
	config.Unmarshal("cache.results.expiry_durations", &conf)
	return &evm{
		methodTTLConfig: conf,
		jrpcSchema:      jrpcSchema,
		selector:        selector,
	}
}

// ethereum chain adapter
type evm struct {
	methodTTLConfig map[string]string
	jrpcSchema      *rpc.JSONRPCSchema
	selector        endpoint.Selector
}

func (a *evm) ChainType() endpoint.ChainType {
	return endpoint.ChainTypeEvm
}

func (a *evm) Select(ctx context.Context, rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint, jsonrpcs []rpc.JSONRPCer) ([]*endpoint.Endpoint, bool) {
	return a.selector.Select(ctx, rc, endpoints, jsonrpcs)
}

func (a *evm) ValidateRequest(jsonrpc rpc.JSONRPCer) error {
	return a.jrpcSchema.ValidateRequest(jsonrpc.Method(), jsonrpc.Map())
}

func (a *evm) ValidateResult(jsonrpc rpc.JSONRPCer, result rpc.JSONRPCResulter, options ...bool) error {
	return a.jrpcSchema.ValidateResponse(jsonrpc.Method(), result.Map(), options...)
}

func (a *evm) _CacheKey(chainId common.ChainId, jsonrpc rpc.JSONRPCer) string {
	params := jsonrpc.Params()
	_params := ""
	if b, err := sonic.Marshal(params); err == nil && len(b) > 0 {
		_params = helpers.Short(string(b))
	}
	return strings.Join([]string{strconv.FormatUint(chainId, 36), jsonrpc.Method(), _params}, ":")
}

// 根据cache配置决定是否缓存，缓存过期时间，是否压缩
func (a *evm) WithCache(chainId common.ChainId, jsonrpc rpc.JSONRPCer) (ok bool, key string, ttl time.Duration) {
	var (
		// not cache tags
		notCacheTags = []string{"earliest", "latest", "pending"}
		d            = a.methodTTLConfig[jsonrpc.Method()]
	)

	if d == "" {
		return false, key, 0.0
	}

	duration, err := time.ParseDuration(d)
	if err != nil {
		return true, key, time.Duration(0)
	}

	switch jsonrpc.Method() {
	case "eth_getBlockByNumber":
		if len(jsonrpc.Params()) >= 1 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
		}
	case "eth_getTransactionByBlockNumberAndIndex":
		if len(jsonrpc.Params()) >= 1 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
		}
	case "eth_getUncleByBlockNumberAndIndex":
		if len(jsonrpc.Params()) >= 1 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
		}
	case "eth_getUncleCountByBlockNumber":
		if len(jsonrpc.Params()) >= 1 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
		}
	case "eth_getBlockTransactionCountByNumber":
		if len(jsonrpc.Params()) >= 1 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
		}
	case "eth_getTransactionCount":
		if len(jsonrpc.Params()) >= 2 {
			ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[1]))
		}
	case "eth_getLogs":
		params := jsonrpc.Params()
		for i := range params {
			param := params[i].(map[string]any)
			fromBlock := fmt.Sprint(param["fromBlock"])
			toBlock := fmt.Sprint(param["toBlock"])

			if slice.Contain(notCacheTags, fromBlock) || slice.Contain(notCacheTags, toBlock) {
				ok = false
				break
			}
		}
	default:
		ok = true
	}

	if !ok {
		return ok, key, time.Duration(0)
	}

	key = a._CacheKey(chainId, jsonrpc)
	return ok, key, duration
}

func (a *evm) GetEndpointValue(jsonrpc rpc.JSONRPCer, endpoints []*endpoint.Endpoint) (found bool, value any, err error) {
	// get block number from endpoint if jsonrpc is eth_blockNumber
	if jsonrpc.Method() == "eth_blockNumber" {
		found = true
		endpoint := slices.MaxFunc(endpoints, func(a *endpoint.Endpoint, b *endpoint.Endpoint) int {
			return int(b.BlockNumber() - a.BlockNumber())
		})
		if height := endpoint.BlockNumber(); height > 0 {
			value = height
		}
	}
	return found, value, nil
}

func (a *evm) SetEndpointValue(jsonrpc rpc.JSONRPCer, _endpoint *endpoint.Endpoint, value any) (err error) {
	if jsonrpc.Method() == "eth_blockNumber" {
		_endpoint.Update(endpoint.WithAttr(endpoint.AttributeBlockNumber, value))
	}
	return nil
}
