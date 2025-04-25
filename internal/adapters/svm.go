package adapters

import (
	"context"
	"fmt"
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

func NewSvmAdapter(
	config *config.Conf,
	selector endpoint.Selector,
) endpoint.ChainAdapter {
	conf := map[string]string{}
	config.Unmarshal("svm.cache_results_duration", &conf)
	return &svm{
		methodTTLConfig: conf,
		selector:        selector,
	}
}

type svm struct {
	methodTTLConfig map[string]string
	selector        endpoint.Selector
}

func (a *svm) ChainType() endpoint.ChainType {
	return endpoint.ChainTypeSvm
}

func (a *svm) Select(ctx context.Context, rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint, jsonrpcs []rpc.JSONRPCer) ([]*endpoint.Endpoint, bool) {
	return a.selector.Select(ctx, rc, endpoints, jsonrpcs)
}

func (a *svm) ValidateRequest(jsonrpc rpc.JSONRPCer) error {
	return nil
}

func (a *svm) ValidateResult(jsonrpc rpc.JSONRPCer, result rpc.JSONRPCResulter, options ...bool) error {
	return nil
}

func (a *svm) _CacheKey(chainId common.ChainId, jsonrpc rpc.JSONRPCer) string {
	params := jsonrpc.Params()
	_params := ""
	if b, err := sonic.Marshal(params); err == nil && len(b) > 0 {
		_params = helpers.Short(string(b))
	}
	return strings.Join([]string{strconv.FormatUint(chainId, 36), jsonrpc.Method(), _params}, ":")
}

// 根据cache配置决定是否缓存，缓存过期时间，是否压缩
func (a *svm) WithCache(chainId common.ChainId, jsonrpc rpc.JSONRPCer) (ok bool, key string, ttl time.Duration) {
	var (
		// 这三个标签不需要缓存
		notCacheCommitments = []string{"processed", "confirmed"}
		d                   = a.methodTTLConfig[jsonrpc.Method()]
	)

	// not cache method
	// getLatestBlockhash
	// getSlot
	if d == "" {
		return false, key, 0.0
	}

	duration, err := time.ParseDuration(d)
	if err != nil {
		return true, key, time.Duration(0)
	}

	switch jsonrpc.Method() {
	// handle opeions commitment and minContextSlot at params[0]
	case "getSlotLeader":
		fallthrough
	case "getSlotLeaders":
		fallthrough
	case "getEpochInfo":
		fallthrough
	case "getTransactionCount":
		fallthrough
	case "getBlockHeight":
		params := jsonrpc.Params()
		if len(params) >= 1 {
			options := params[0].(map[string]any)
			if options["minContextSlot"] != nil {
				ok = true
				duration *= 2
			} else if options["commitment"] != nil {
				ok = !slice.Contain(notCacheCommitments, fmt.Sprint(options["commitment"]))
			}
		}
	// handle opeions commitment and minContextSlot at params[1]
	case "getProgramAccounts":
		fallthrough
	case "getSignaturesForAddress":
		fallthrough
	case "getMultipleAccounts":
		fallthrough
	case "getInflationReward":
		fallthrough
	case "getTransaction":
		fallthrough
	case "getFeeForMessage":
		fallthrough
	case "getAccountInfo":
		fallthrough
	case "getBalance":
		params := jsonrpc.Params()
		if len(params) >= 2 {
			options := params[1].(map[string]any)
			if options["minContextSlot"] != nil {
				ok = true
				duration *= 2
			} else if options["commitment"] != nil {
				ok = !slice.Contain(notCacheCommitments, fmt.Sprint(options["commitment"]))
			}
		}
		// handle opeions commitment and minContextSlot at params[2]
	case "getTokenAccountsByOwner":
		params := jsonrpc.Params()
		if len(params) >= 3 {
			options := params[2].(map[string]any)
			if options["minContextSlot"] != nil {
				ok = true
				duration *= 2
			} else if options["commitment"] != nil {
				ok = !slice.Contain(notCacheCommitments, fmt.Sprint(options["commitment"]))
			}
		}
	// handle commitment at params[0]
	case "getLeaderSchedule":
		fallthrough
	case "getTokenAccountBalance":
		fallthrough
	case "getVoteAccounts":
		fallthrough
	case "getTokenLargestAccounts":
		fallthrough
	case "getInflationGovernor":
		fallthrough
	case "getBlockProduction":
		fallthrough
	case "getTokenSupply":
		params := jsonrpc.Params()
		if len(params) >= 1 {
			options := params[0].(map[string]any)
			if options["commitment"] != nil {
				ok = !slice.Contain(notCacheCommitments, fmt.Sprint(options["commitment"]))
			}
		}
	// handle commitment at params[1]
	case "getMinimumBalanceForRentExemption":
		params := jsonrpc.Params()
		if len(params) >= 2 {
			options := params[1].(map[string]any)
			if options["commitment"] != nil {
				ok = !slice.Contain(notCacheCommitments, fmt.Sprint(options["commitment"]))
			}
		}
	// getHealth
	// getBlock
	// getBlockCommitment
	// getInflationRate
	// getBlocks
	// getSlotLeaders
	// getBlocksWithLimit
	// getEpochSchedule
	// getBlockTime
	// getClusterNodes
	// getFirstAvailableBlock
	// getGenesisHash
	// getVersion
	// getRecentPerformanceSamples
	// minimumLedgerSlot
	// getSupply
	// getStakeMinimumDelegation
	// getSignatureStatuses
	// getRecentPrioritizationFees
	// getMaxShredInsertSlot
	// getMaxRetransmitSlot
	// getIdentity
	// getHighestSnapshotSlot
	default:
		ok = true
	}

	if !ok {
		return ok, key, time.Duration(0)
	}
	key = a._CacheKey(chainId, jsonrpc)
	return ok, key, duration
}

func (a *svm) GetEndpointValue(jsonrpc rpc.JSONRPCer, endpoints []*endpoint.Endpoint) (found bool, data any, err error) {
	return false, nil, nil
}

func (a *svm) SetEndpointValue(jsonrpc rpc.JSONRPCer, _endpoint *endpoint.Endpoint, value any) (err error) {
	return nil
}
