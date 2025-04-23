package adapters

import (
	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"go.uber.org/fx"
)

// register bulky of agent module
var NewAdapterModule = fx.Options(
	fx.Provide(NewChainAdapters),

	fx.Provide(fx.Annotate(
		NewEvmAdapter,
		fx.ResultTags(`group:"chain-adapters"`),
	)),

	fx.Provide(fx.Annotate(
		NewSvmAdapter,
		fx.ResultTags(`group:"chain-adapters"`),
	)),
)

// 收集所有实现 MyInterface 的对象
type AdapterCollector struct {
	fx.In
	Adapters []endpoint.ChainAdapter `group:"chain-adapters"`
}

func NewChainAdapters(collerctor AdapterCollector) map[string]endpoint.ChainAdapter {
	adapters := map[string]endpoint.ChainAdapter{}
	for _, adapter := range collerctor.Adapters {
		adapters[adapter.ChainType()] = adapter
	}
	return adapters
}
