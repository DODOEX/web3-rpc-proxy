package app

import (
	"github.com/DODOEX/web3rpcproxy/internal/app/agent/controller"
)

type Router struct {
	app *Application
	Agent controller.AgentController
	Other controller.OtherController
}

func NewRouter(
	app *Application,
	agent controller.AgentController,
	other controller.OtherController,
) *Router {
	return &Router{
		app:   app,
		Agent: agent,
		Other: other,
	}
}

// register routes of agent module
func (c *Router) RegisterRoutes() {
	// define routes
	c.app.Router.GET("/metrics", c.Other.HandleMetrics)
	c.app.Router.GET("/k8s/healthz", c.Other.HandleK8sHealthz)

	c.app.Router.POST("/{chain}", c.Agent.HandleCall)
	c.app.Router.POST("/{apikey}/{chain}", c.Agent.HandleCall)
	c.app.Router.POST("/rpc/{chain}", c.Agent.HandleCall)
}
