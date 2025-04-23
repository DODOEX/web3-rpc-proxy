package endpoint

import (
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
)

// 新增链类型抽象层
type ChainAdapter interface {
	Selector
	ChainType() ChainType // e.g. "eth", "solana", "ton"
	ValidateRequest(rpc.JSONRPCer) error
	ValidateResult(rpc.JSONRPCer, rpc.JSONRPCResulter, ...bool) error
	WithCache(common.ChainId, rpc.JSONRPCer) (yes bool, key string, ttl time.Duration)
	GetEndpointValue(rpc.JSONRPCer, []*Endpoint) (found bool, data any, err error)
	SetEndpointValue(rpc.JSONRPCer, *Endpoint, any) error
}

type ChainType = string

const (
	ChainTypeEvm ChainType = "evm"
	ChainTypeSvm ChainType = "svm"
	ChainTypeTvm ChainType = "tvm"
)
