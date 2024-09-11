package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/allegro/bigcache"
	"github.com/duke-git/lancet/v2/slice"
)

func _CacheKey(chainId common.ChainId, jsonrpc rpc.JSONRPCer) string {
	params := jsonrpc.Raw()["params"]
	if params == nil {
		params = []any{}
	}
	_params := ""
	if b, err := json.Marshal(params); err == nil && len(b) > 0 {
		_params = helpers.Short(string(b))
	}
	return strings.Join([]string{strconv.FormatUint(chainId, 36), jsonrpc.Method(), _params}, ":")
}

// 根据cache配置决定是否缓存，缓存过期时间，是否压缩
func _WithCache(config map[string]string, jsonrpc rpc.JSONRPCer) (ok bool, ttl time.Duration) {
	var (
		// 这三个标签不需要缓存
		notCacheTags = []string{"earliest", "latest", "pending"}
		v            = config[jsonrpc.Method()]
	)

	if v != "" {
		ok := true
		switch jsonrpc.Method() {
		case "eth_getBlockByNumber":
			if len(jsonrpc.Params()) >= 1 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
			} else {
				ok = false
			}
		case "eth_getTransactionByBlockNumberAndIndex":
			if len(jsonrpc.Params()) >= 1 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
			} else {
				ok = false
			}
		case "eth_getUncleByBlockNumberAndIndex":
			if len(jsonrpc.Params()) >= 1 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
			} else {
				ok = false
			}
		case "eth_getUncleCountByBlockNumber":
			if len(jsonrpc.Params()) >= 1 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
			} else {
				ok = false
			}
		case "eth_getBlockTransactionCountByNumber":
			if len(jsonrpc.Params()) >= 1 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[0]))
			} else {
				ok = false
			}
		case "eth_getTransactionCount":
			if len(jsonrpc.Params()) >= 2 {
				ok = !slice.Contain(notCacheTags, fmt.Sprint(jsonrpc.Params()[1]))
			} else {
				ok = false
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
		}

		if d, err := time.ParseDuration(v); err == nil {
			return ok, d
		}
		return ok, time.Duration(0)
	}

	return false, 0.0
}

func _SetCache(cache *bigcache.BigCache, k string, v any) error {
	if data, err := json.Marshal(v); err == nil {
		err = cache.Set(k, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func _GetCache[T any](cache *bigcache.BigCache, k string, i T) error {
	data, err := cache.Get(k)
	if err == nil {
		err = json.Unmarshal(data, i)
	}
	return err
}
