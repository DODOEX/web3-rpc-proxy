package agent

import (
	"github.com/DODOEX/web3rpcproxy/internal/app/agent/controller"
	"github.com/DODOEX/web3rpcproxy/internal/app/agent/repository"
	"github.com/DODOEX/web3rpcproxy/internal/app/agent/service"
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

	// register controller of agent module
	fx.Provide(controller.NewAgentController),
	fx.Provide(controller.NewOtherController),
)
