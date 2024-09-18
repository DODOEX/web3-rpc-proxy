package bootstrap

import (
	"context"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/application"
	"github.com/DODOEX/web3rpcproxy/internal/database"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/controller"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/service"
	"github.com/DODOEX/web3rpcproxy/internal/module/core"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/module/shared"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/rs/zerolog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"
	"gopkg.in/yaml.v2"

	fxzerolog "github.com/efectn/fx-zerolog"
	"github.com/prometheus/client_golang/prometheus"
)

func StartCluster() {
	// 注册指标
	prometheus.MustRegister(utils.TotalRequests)
	prometheus.MustRegister(utils.RequestDurations)
	prometheus.MustRegister(utils.TotalEndpoints)
	prometheus.MustRegister(utils.EndpointDurations)
	prometheus.MustRegister(utils.TotalCaches)
	prometheus.MustRegister(utils.TotalAmqpMessages)

	fx.New(
		// provide modules
		shared.NewSharedModule,
		core.NewCoreModule,
		agent.NewAgentModule,

		// application
		fx.Provide(application.NewApplication),

		// define options
		fx.WithLogger(fxzerolog.Init()),
		fx.StartTimeout(5*time.Minute),
		fx.StopTimeout(5*time.Minute),

		// launch
		fx.Invoke(InitCluster),
	).Run()
}

// function to start webserver
func InitCluster(
	lifecycle fx.Lifecycle,
	conf *config.Conf,
	logger zerolog.Logger,
	database *database.Database,
	amqp *shared.Amqp,
	etcd *clientv3.Client,
	redis *shared.RedisClient,
	watcher *shared.WatcherClient,
	ecf *endpoint.ClientFactory,
	controller *controller.Controller,
	app *application.Application,
	service service.EndpointService,
) {
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				var i = 1
				logger = logger.With().Str("name", "cluster").Logger()

				if err := database.Connect(ctx); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error interrupted when to connect the Database!", i)
				} else {
					logger.Info().Msgf("%d- Connected the Database succesfully!", i)
				}
				i++

				if err := redis.Connect(ctx); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error interrupted when to connect the Redis!", i)
				} else {
					logger.Info().Msgf("%d- Connected the Redis succesfully!", i)
				}
				i++

				if err := amqp.Connect(ctx); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error occurred when to connect the Amqp!", i)
				} else {
					logger.Info().Msgf("%d- Connected the Amqp succesfully!", i)
				}
				i++

				// 监控 endpoints.yaml 配置文件
				if conf.Exists(shared.KoanfEtcdEndpointsConfigToken) {
					watcher.OnChanaged(conf.String(shared.KoanfEtcdEndpointsConfigToken), func(path string, value []byte) {
						var (
							val = []any{}
							err = yaml.Unmarshal(value, &val)
						)
						if err != nil {
							logger.Fatal().Msgf("Error loading watch config: %v", err)
						} else {
							conf.Set(shared.KoanfEndpointsToken, val)
							config.LoadEndpointChains(conf, shared.KoanfEndpointsToken)
							service.Purge()
							logger.Printf(conf.Sprint())
							logger.Printf("Reload %s config.", path)
						}
					})
					logger.Info().Msgf("%d- Watching endpoints.yaml config file...", i)
				} else {
					logger.Warn().Msgf("%d- Watch endpoints.yaml config file is disabled!", i)
				}

				go func() {
					service.Init()
					controller.RegisterRoutes()

					logger.Info().Msg("🚀 " + app.AppName + " is running! listen on http://" + app.Hostname + ":" + app.Port)
					if err := app.Run(); err != nil {
						logger.Error().Err(err).Msg("An unknown error occurred when to run server!")
					}
				}()

				return nil
			},
			OnStop: func(ctx context.Context) error {
				logger.Info().Msg("Running cleanup tasks...")

				var i = 1

				if err := app.Shutdown(ctx); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error occurred when to shutdown the Server!", i)
				} else {
					logger.Info().Msgf("%d- Shutdown the Server succesfully!", i)
				}
				i++

				if err := etcd.Close(); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error occurred when to closed the etcd!", i)
				} else {
					logger.Info().Msgf("%d- Closed the ETCD succesfully!", i)
				}
				i++

				if err := redis.Close(); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error occurred when to closed the redis!", i)
				} else {
					logger.Info().Msgf("%d- Closed the Redis succesfully!", i)
				}
				i++

				if err := amqp.Close(); err != nil {
					logger.Error().Err(err).Msgf("%d- An unknown error occurred when to closed the amqp!", i)
				} else {
					logger.Info().Msgf("%d- Closed the Amqp succesfully!", i)
				}
				i++

				if err := database.Close(); err != nil {
					logger.Error().Err(err).Msg("An unknown error occurred when to shutdown the database!")
				} else {
					logger.Info().Msgf("%d- Closed the Database succesfully!", i)
				}
				i++

				logger.Info().Msgf("%s was successful shutdown.", app.AppName)
				logger.Info().Msg("\u001b[96msee you again👋\u001b[0m")

				return nil
			},
		},
	)
}
