package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/application"
	"github.com/DODOEX/web3rpcproxy/internal/database"
	"github.com/DODOEX/web3rpcproxy/internal/module/shared"
	prometheusfasthttp "github.com/gohutool/boot4go-prometheus/fasthttp"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type otherController struct {
	logger  zerolog.Logger
	amqp    *shared.Amqp
	rclient *shared.RedisClient
	db      *database.Database
}

type OtherController interface {
	HandleK8sHealthz(ctx *fasthttp.RequestCtx)
	HandleMetrics(ctx *fasthttp.RequestCtx)
}

func NewOtherController(
	logger zerolog.Logger,
	app *application.Application,
	amqp *shared.Amqp,
	rclient *shared.RedisClient,
	db *database.Database,
) OtherController {
	controller := &otherController{
		logger:  logger.With().Str("name", "other_controller").Logger(),
		amqp:    amqp,
		rclient: rclient,
		db:      db,
	}

	return controller
}

func (o *otherController) HandleK8sHealthz(ctx *fasthttp.RequestCtx) {
	var (
		_ctx, cancel = context.WithTimeoutCause(ctx, 3*time.Second, nil) // fasthttp 默认超时 3s
		wg           sync.WaitGroup
		err          error
	)

	wg.Add(1)
	go func() {
		defer wg.Done()

		if _err := o.rclient.Client.Ping(_ctx).Err(); _err != nil {
			if err != nil {
				return
			}
			err = _err
			o.logger.Error().Stack().Err(_err).Send()
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if _err := o.db.DB.WithContext(_ctx).Raw("SELECT 1").Error; _err != nil {
			if err != nil {
				return
			}
			err = _err
			o.logger.Error().Stack().Err(_err).Send()
			cancel()
		}
	}()

	if o.amqp.Conn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if o.amqp.Conn.IsClosed() {
				if err != nil {
					return
				}
				err = fmt.Errorf("amqp connection is closed")
				o.logger.Error().Stack().Err(err).Send()
				cancel()
				return
			}
		}()
	}

	wg.Wait()
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
	} else {
		ctx.Success("application/text", []byte("ok"))
	}
}

func (o *otherController) HandleMetrics(ctx *fasthttp.RequestCtx) {
	prometheusfasthttp.PrometheusHandler(prometheusfasthttp.HandlerOpts{})(ctx)
}
