package agent

import (
	"net/http"

	"github.com/DODOEX/web3rpcproxy/internal/module/agent/controller"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/repository"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/service"
	web3rpcprovider "github.com/DODOEX/web3rpcproxy/providers/web3-rpc-provider"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"go.uber.org/fx"
)

// register bulky of agent module
var NewAgentModule = fx.Options(
	// register repository of agent module
	fx.Provide(repository.NewTenantRepository),

	// register service of agent module
	fx.Provide(service.NewAgentService),
	fx.Provide(service.NewTenantService),
	fx.Provide(service.NewEndpointService),
	fx.Provide(NewWeb3RPCProvider),

	// register controller of agent module
	fx.Provide(controller.NewAgentController),
	fx.Provide(controller.NewOtherController),

	fx.Provide(controller.NewController),
)

type Web3RPCProviderConfig struct {
	Method string `yaml:"method" koanf:"method"`
	Url string `yaml:"url" koanf:"url"`
	Headers map[string]string `yaml:"headers" koanf:"headers"`
	Sources []string `yaml:"sources" koanf:"sources"`
}

func NewWeb3RPCProvider(config *config.Conf) *web3rpcprovider.Web3RPCProvider {
	c := Web3RPCProviderConfig{Method: "GET"}
	config.Unmarshal("providers.web3-rpc-provider", &c)

	if c.Url == "" {
		return nil
	}

	req, err := http.NewRequest(c.Method, c.Url, nil)
	if err != nil {
		return nil
	}

	if c.Headers != nil {
		headers := make(http.Header)
		for key, value := range c.Headers {
			headers.Set(key, value)
		}
		req.Header = headers
	}

	_config := &web3rpcprovider.Web3RPCProviderConfig{
		Request: req,
		Client: &http.Client{},
		Sources: c.Sources,
	}
	return web3rpcprovider.NewWeb3RPCProvider(_config)
}
