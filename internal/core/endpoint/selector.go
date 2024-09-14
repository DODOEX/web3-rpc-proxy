package endpoint

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"slices"

	"github.com/DODOEX/web3rpcproxy/internal/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/duke-git/lancet/v2/slice"
)

type RetryStrategy int8

const (
	Same     RetryStrategy = 0 // 总是重试相同的节点
	Rotation RetryStrategy = 1 // 交替重试，可用节点
)

func (s RetryStrategy) String() string {
	switch s {
	case Same:
		return "same"
	case Rotation:
		return "rotation"
	default:
		return "unknown"
	}
}

func ParseRetryStrategy(s string) RetryStrategy {
	switch s {
	case "same":
		return Same
	case "rotation":
		return Rotation
	default:
		return Same
	}
}

type EndpointType = string

const (
	EndpointType_Fullnode   EndpointType = "fullnode"
	EndpointType_Activenode EndpointType = "activenode"
	EndpointType_Default    EndpointType = "default"
)

type Selector interface {
	Select(ctx context.Context, rc reqctx.Reqctxs, endpoints []*Endpoint, jsonrpcs []rpc.JSONRPCer) ([]*Endpoint, bool)
}

type selector struct {
	heightenResponseTime *HeightenResponseTime
}

func NewSelector() Selector {
	return &selector{
		heightenResponseTime: &HeightenResponseTime{},
	}
}

func (s *selector) getArranger() arranger {
	return s.heightenResponseTime
}

// 获取endpoints
func (s *selector) Select(ctx context.Context, rc reqctx.Reqctxs, endpoints []*Endpoint, jsonrpcs []rpc.JSONRPCer) ([]*Endpoint, bool) {
	if len(endpoints) <= 0 {
		return nil, false
	}
	if len(endpoints) <= 1 {
		return endpoints, true
	}

	var _endpoints []*Endpoint
	// 筛选指定类型的节点
	if types := rc.Options().EndpointTypes(); len(types) > 0 {
		_endpoints = slice.Filter(endpoints, func(_ int, e *Endpoint) bool {
			return slices.Index(types, e.Type()) > -1
		})
	}

	// 默认根据请求method, 选择不同类型的节点
	if len(_endpoints) <= 0 {
		methods := []string{
			"eth_getBlockByNumber",
			"eth_getBlockByHash",
			"eth_getTransactionByHash",
			"eth_getTransactionByBlockHashAndIndex",
			"eth_getTransactionByBlockNumberAndIndex",
			"eth_getTransactionReceipt",
			"eth_getTransactionCount",
			"eth_getUncleByBlockHashAndIndex",
			"eth_getUncleByBlockNumberAndIndex",
			"eth_getBlockTransactionCountByHash",
			"eth_getBlockTransactionCountByNumber",
			"eth_getUncleCountByBlockHash",
			"eth_getUncleCountByBlockNumber",
			"eth_blockNumber",
			"eth_accounts",
			"eth_gasPrice",
			"eth_chainId",
			"net_version",
		}

		if slice.Every(jsonrpcs, func(i int, jsonrpc rpc.JSONRPCer) bool {
			return slices.Contains(methods, jsonrpc.Method())
		}) {
			// full node
			_endpoints = slice.Filter(endpoints, func(_ int, e *Endpoint) bool {
				return e.Type() == EndpointType_Fullnode
			})
		}

		// 如果没有命中fullnode类型的节点，则直接选择用其他节点(active node)
		if len(_endpoints) <= 0 {
			_endpoints = endpoints
		}
	}

	arranged, err := s.getArranger().arrange(ctx, _endpoints)
	if err != nil {
		_endpoints = slice.Shuffle(_endpoints)
	} else {
		// 如果允许多次交替重试，则随机将第一个不健康endpoint移动到第一位，便于检查异常endpoint状态
		if arranged[0].Health() && rc.Options().Attempts() > 1 && rc.Options().AttemptStrategy() == reqctx.Rotation {
			// 超时越大，检查不健康endpoint的概率越大；超时越大，用户可容忍请求时间越大
			if rand.Intn(100) < int(float64(rc.Options().Timeout())/float64(reqctx.MaxTimeout)*100) {
				for i, e := range arranged {
					if !e.Health() {
						arranged = append([]*Endpoint{e}, append(arranged[:i], arranged[i+1:]...)...)
						break
					}
				}
			}
		}
		_endpoints = arranged
	}

	return _endpoints, true
}

type arranger interface {
	// 输出一组合适的endpoint
	arrange(ctx context.Context, endpoints []*Endpoint) ([]*Endpoint, error)
}

type HeightenResponseTime struct{}

func normalizeEndpointValues(endpoints []*Endpoint, attrs []EndpointAttribute, scale float64) map[*Endpoint]map[EndpointAttribute]float64 {
	if len(endpoints) == 0 {
		return make(map[*Endpoint]map[EndpointAttribute]float64)
	}
	mins := make(map[EndpointAttribute]float64, len(attrs))
	maxs := make(map[EndpointAttribute]float64, len(attrs))
	for i := range endpoints {
		for _, attr := range attrs {
			v := 0.0
			if _v, ok := helpers.ToFloat(endpoints[i].Read(attr)); ok {
				v = _v
			}
			if maxs[attr] == 0 {
				maxs[attr] = v
			}
			if mins[attr] == 0 {
				mins[attr] = v
			}
			if v < mins[attr] {
				mins[attr] = v
			}
			if v > maxs[attr] {
				maxs[attr] = v
			}
		}
	}

	normalized := make(map[*Endpoint]map[EndpointAttribute]float64, len(endpoints))
	for i := range endpoints {
		normalized[endpoints[i]] = make(map[EndpointAttribute]float64, len(attrs))
		for j := range attrs {
			max, min := maxs[attrs[j]], mins[attrs[j]]
			if (max - min) > 0 {
				v := 0.0
				if _v, ok := helpers.ToFloat(endpoints[i].Read(attrs[j])); ok {
					v = _v
				}
				normalized[endpoints[i]][attrs[j]] = (v - min) / (max - min) * scale
			} else {
				normalized[endpoints[i]][attrs[j]] = 0
			}
		}
	}
	return normalized
}

func calculateEndpointScores(values map[*Endpoint]map[EndpointAttribute]float64) (float64, map[*Endpoint]float64) {
	var (
		total  = 0.0
		scores = make(map[*Endpoint]float64, len(values))
	)

	for endpoint, value := range values {
		// block number > duration | p95duration > weight > count
		score := 0.0

		// Higher score for bigger number of block
		score += value[BlockNumber] * 2

		// Higher score for lower duration or p99 duration
		score += 100 - math.Min(value[Duration], value[P95Duration])

		// Higher score for lower total requests (to balance the load)
		score += 100 - (value[Count] * 1.1)

		// Higher score for bigger wight
		score += value[Weight]

		if score < 0 {
			score = 0
		}

		total += score
		scores[endpoint] = score
	}

	return total, scores
}

// 最佳节点排序
func best(scores map[*Endpoint]float64) func(a, b *Endpoint) bool {
	return func(a, b *Endpoint) bool {
		aHealth, bHealth := a.Health(), b.Health()
		if aHealth && !bHealth {
			return true
		}
		if !aHealth && bHealth {
			return false
		}

		aP95Health, bP95Health := a.P95Health(), b.P95Health()
		if aP95Health && !bP95Health {
			return true
		}
		if !aP95Health && bP95Health {
			return false
		}

		if scores[a] > scores[b] {
			return true
		}
		if scores[a] < scores[b] {
			return false
		}

		aLastUpdateTime, bLastUpdateTime := a.LastUpdateTime(), b.LastUpdateTime()
		if aLastUpdateTime.Before(bLastUpdateTime) {
			return true
		}
		if aLastUpdateTime.After(bLastUpdateTime) {
			return false
		}

		return true
	}
}

func (h *HeightenResponseTime) arrange(ctx context.Context, endpoints []*Endpoint) ([]*Endpoint, error) {
	if len(endpoints) <= 1 {
		return endpoints, nil
	}

	values := normalizeEndpointValues(endpoints, []EndpointAttribute{BlockNumber, Duration, P95Duration, Count, Weight}, 100)
	total, scores := calculateEndpointScores(values)

	if total <= 0 {
		return nil, errors.New("cannot calculate endpoints")
	}

	slice.SortBy(endpoints, best(scores))

	return endpoints, nil
}
