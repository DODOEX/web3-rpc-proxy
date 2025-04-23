package core

import (
	"net/http"

	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"go.uber.org/fx"
)

// register bulky of agent module
var NewCoreModule = fx.Options(
	fx.Provide(NewClient),

	fx.Provide(endpoint.NewSelector),

	fx.Provide(NewJSONRPCSchema),

	fx.Provide(NewClientFactory),
)

func NewClientFactory(config *config.Conf, t *http.Transport, jrpcSchema *rpc.JSONRPCSchema) *endpoint.ClientFactory {
	_config := &endpoint.ClientFactoryConfig{
		ClientsSize:   config.Int("clients.size", 64),
		JSONRPCSchema: jrpcSchema,
		Transport:     t,
	}
	return endpoint.NewClientFactory(_config)
}

func NewJSONRPCSchema(config *config.Conf) *rpc.JSONRPCSchema {
	if config.Bool("jsonrpc.enable_validation", false) {
		b := config.Get("jsonrpc.schema")
		if v, ok := b.([]byte); ok {
			return rpc.NewJSONRPCSchema(v)
		}
	}

	return rpc.NewJSONRPCSchema([]byte{})
}
