package controller

import "github.com/DODOEX/web3rpcproxy/internal/application"

type Controller struct {
	app   *application.Application
	Agent AgentController
	Other OtherController
}

func NewController(
	app *application.Application,
	agent AgentController,
	other OtherController,
) *Controller {
	return &Controller{
		app:   app,
		Agent: agent,
		Other: other,
	}
}

// register routes of agent module
func (c *Controller) RegisterRoutes() {
	// define routes
	c.app.Router.GET("/metrics", c.Other.HandleMetrics)
	c.app.Router.GET("/k8s/healthz", c.Other.HandleK8sHealthz)

	c.app.Router.POST("/{chain}", c.Agent.HandleCall)
	c.app.Router.POST("/{apikey}/{chain}", c.Agent.HandleCall)
	c.app.Router.POST("/rpc/{chain}", c.Agent.HandleCall)
}
