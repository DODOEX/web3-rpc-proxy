package app

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/prefork"
)

type Application struct {
	logger            zerolog.Logger
	Router            *router.Router
	s                 *fasthttp.Server
	IdleTimeout       time.Duration
	AppName           string
	Network           string
	Hostname          string
	Port              string
	Concurrency       int
	MaxConnsPerIP     int
	Prefork           bool
	EnablePrintRoutes bool
	Production        bool
	ReduceMemoryUsage bool
	TCPKeepalive      bool
}

func NewApplication(logger zerolog.Logger, conf *config.Conf) *Application {
	hostname, port := config.ParseAddress(conf.String("app.host", "0.0.0.0:8080"))
	if hostname == "" {
		if conf.String("app.network", "tcp4") == "tcp6" {
			hostname = "[::1]"
		} else {
			hostname = "0.0.0.0"
		}
	}
	application := &Application{
		logger:            logger,
		Router:            router.New(),
		Production:        conf.Bool("app.production", false),
		EnablePrintRoutes: conf.Bool("app.print-routes", true),
		AppName:           conf.String("app.name", "Web3 RPC Proxy"),
		Hostname:          hostname,
		Port:              port,
		Prefork:           conf.Bool("app.prefork", false),
		Concurrency:       conf.Int("app.concurrency", fasthttp.DefaultConcurrency),
		IdleTimeout:       conf.Duration("app.idle-timeout", 30*time.Second),
		ReduceMemoryUsage: conf.Bool("app.reduce-memory-usage"),
		TCPKeepalive:      conf.Bool("app.tcp-keepalive"),
		MaxConnsPerIP:     conf.Int("app.max-conns-per-ip"),
	}

	return application
}

func (a *Application) HandlersCount() int {
	m := a.Router.List()
	c := 0
	for k := range m {
		c += len(m[k])
	}
	return c
}

func (a *Application) Run() error {
	a.s = &fasthttp.Server{
		Name:              a.AppName,
		Handler:           a.Router.Handler,
		Concurrency:       a.Concurrency,
		IdleTimeout:       a.IdleTimeout,
		ReduceMemoryUsage: a.ReduceMemoryUsage,
		TCPKeepalive:      a.TCPKeepalive,
		MaxConnsPerIP:     a.MaxConnsPerIP,
		CloseOnShutdown:   true,
	}

	a.Router.PanicHandler = func(ctx *fasthttp.RequestCtx, rcv any) {
		a.logger.Error().Stack().Err(rcv.(error)).Msgf("Panic occurred")
	}

	// Debug informations
	if !a.Production {
		prefork := "Enabled"
		procs := runtime.GOMAXPROCS(0)
		if !a.Prefork {
			procs = 1
			prefork = "Disabled"
		}

		a.logger.Debug().Msgf("Version: %s", "-")
		a.logger.Debug().Msgf("Hostname: %s", a.Hostname)
		a.logger.Debug().Msgf("Port: %s", a.Port)
		a.logger.Debug().Msgf("Prefork: %s", prefork)
		a.logger.Debug().Msgf("Handlers: %d", a.HandlersCount())
		a.logger.Debug().Msgf("Processes: %d", procs)
		a.logger.Debug().Msgf("PID: %d", os.Getpid())
	}

	if a.Prefork {
		preforkServer := prefork.New(a.s)

		return preforkServer.ListenAndServe(a.Hostname + ":" + a.Port)
	}

	return a.s.ListenAndServe(a.Hostname + ":" + a.Port)
}

func (a *Application) Shutdown(ctx context.Context) error {
	return a.s.ShutdownWithContext(ctx)
}
