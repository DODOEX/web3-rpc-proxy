package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/application"
	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/service"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/internal/module/shared"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/rs/zerolog"
	"github.com/streadway/amqp"
	"github.com/valyala/fasthttp"
)

type agentController struct {
	logger          zerolog.Logger
	config          *config.Conf
	amqp            *shared.Amqp
	application     *application.Application
	agentService    service.AgentService
	tenantService   service.TenantService
	endpointService service.EndpointService
}

type AgentController interface {
	HandleCall(ctx *fasthttp.RequestCtx)
}

func NewAgentController(
	logger zerolog.Logger,
	config *config.Conf,
	application *application.Application,
	amqp *shared.Amqp,
	agentService service.AgentService,
	tenantService service.TenantService,
	endpointService service.EndpointService,
) AgentController {
	controller := &agentController{
		application:     application,
		config:          config,
		amqp:            amqp,
		logger:          logger.With().Str("name", "agent_controller").Logger(),
		agentService:    agentService,
		tenantService:   tenantService,
		endpointService: endpointService,
	}

	return controller
}

// get all agents
// @Summary      Get all agents
// @Description  API for getting all agents
// @Tags         Task
// @Security     Bearer
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Failure      422  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /agents [get]
func (a *agentController) HandleCall(ctx *fasthttp.RequestCtx) {
	// 返回结果
	var (
		body       = []byte{}
		status     = common.Error
		statusCode = 500
	)

	rc, chainCode := a.getRequestContext(ctx), "unknown"
	if endpoints, ok := a.endpointService.GetAll(rc.Chain().ID); !ok || len(endpoints) <= 0 {
		// 暂不支持该链
		rc.Logger().Warn().Msgf("Unsupport chain: %s", fmt.Sprint(ctx.UserValue("chain")))
		err := common.NotFoundError("Unsupported")

		status = err.QueryStatus()
		statusCode = err.StatusCode()
		body = err.Body()
	} else {
		chainCode = rc.Chain().Code

		// 处理请求
		data, err := a.call(rc, endpoints)

		if err != nil {
			status = err.QueryStatus()
			statusCode = err.StatusCode()
			body = err.Body()
		} else {
			statusCode = http.StatusOK
			status = common.Success
			body = data
		}
	}

	defer func() {
		if !ctx.Response.ConnectionClose() {
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "*")
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "*")
			ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			ctx.Response.Header.Set("Access-Control-Expose-Headers", "*")
			ctx.Response.Header.Set("Referrer-Policy", "same-origin")
			ctx.Response.Header.Set("Server", a.application.AppName)
			ctx.Response.Header.SetContentType("application/json; charset=utf-8")
			ctx.SetBody(body)
			ctx.SetStatusCode(statusCode)
		}
	}()

	app, chain, p := rc.App(), rc.Chain(), rc.Profile()
	// 记录
	p.Status = status
	p.Endtime = time.Now().UnixMilli()

	// 请求后的补偿
	if app != nil {
		if p.Status == common.Success || p.Status == common.Fail {
			// 后端节点报错，也算正常消费
			go a.tenantService.Affected(app)
		} else {
			// 如果内部错误，则不算消费
			go a.tenantService.Unaffected(app)
		}
	}

	// 上报
	if a.amqp.Conn != nil && chain.ID != 0 && p != nil {
		go a.publish(chain, app, p)
	}

	appName := "unknown"
	if app != nil {
		appName = app.Name
	}
	utils.TotalRequests.WithLabelValues(chainCode, appName, string(status)).Inc()
	utils.RequestDurations.WithLabelValues(chainCode, appName).Observe(float64(p.Endtime-p.Starttime) / 1000.0)
	rc.Logger().Info().Any("status", status).TimeDiff("ms", time.UnixMilli(p.Endtime), time.UnixMilli(p.Starttime)).Msgf("%s %s %d", ctx.Method(), ctx.RequestURI(), statusCode)
}

func (a agentController) call(rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint) ([]byte, common.HTTPErrors) {
	ctx, cancel := context.WithTimeoutCause(rc, rc.Options().Timeout(), common.TimeoutError("Request timed out"))
	defer cancel()

	// 解析 app
	if rc.App() == nil {
		app, err := a.getTenantApp(ctx, rc)
		if common.IsHTTPErrors(err) {
			rc.Logger().Error().Str(zerolog.ErrorFieldName, err.(common.HTTPErrors).String()).Send()
			return nil, err.(common.HTTPErrors)
		} else if err != nil {
			rc.Logger().Error().Err(err).Send()
			return nil, common.InternalServerError("", err)
		}
		rc.SetApp(app)
	}

	// 调用
	data, err := a.agentService.Call(ctx, rc, endpoints)

	if common.IsHTTPErrors(err) {
		rc.Logger().Error().Str(zerolog.ErrorFieldName, err.(common.HTTPErrors).String()).Send()
		return nil, err.(common.HTTPErrors)
	} else if err != nil {
		rc.Logger().Error().Stack().Err(err).Send()
		return nil, common.InternalServerError("", err)
	}

	return data, nil
}

func (a agentController) getTenantApp(ctx context.Context, reqctx reqctx.Reqctxs) (*common.App, error) {
	token := reqctx.AppKey()
	if len(token) <= 0 {
		return nil, common.ForbiddenError("Token is empty")
	}

	app, err := a.tenantService.Access(ctx, token, reqctx.AppBucket())
	if err != nil {
		reqctx.Logger().Error().Stack().Err(err).Msg("Get app error")
		if err.Error() == "context deadline exceeded" {
			if cause := context.Cause(ctx); cause != nil && common.IsHTTPErrors(cause) {
				return nil, cause
			}
		}

		return nil, common.ForbiddenError("Token is invalid")
	}

	// 拒绝访问
	if app.Balance <= -1 {
		// const overview = `⏳ ${app.balance}/${app.capacity} | ♻️ ${app.rate}/s | 🕛 ${app.last}`;
		// const key = hidePrivacyInfo(app.token) + (app.bucket ? ', ' + hidePrivacyInfo(app.bucket) : '');
		// this.logger.Warn(`[${ctx.state.id}] ${app.name}(${key}) requests overage. ${overview}`);
		reqctx.Logger().Warn().Msgf("proxy overage. ⏳ %d/%f | ♻️ %f/s", app.Balance, app.Capacity, app.Rate)
		return nil, common.TooManyRequestsError("Token is overage")
	}

	return app, nil
}

// func (_i agentService) getRequestContext(ctx *fasthttp.RequestCtx, c net.Conn) reqctx.Reqctxs {
func (a agentController) getRequestContext(ctx *fasthttp.RequestCtx) reqctx.Reqctxs {
	return reqctx.NewReqctx(ctx, a.config.Copy(), a.logger)
}

func (a agentController) publish(chain common.Chain, app *common.App, data *common.QueryProfile) {
	defer func() {
		if err := recover(); err != nil {
			a.logger.Error().Interface("error", err).Msg("Failed to publish to amqp")
		}
	}()

	if a.amqp.Conn.IsClosed() {
		a.logger.Error().Msg("connection is closed, skip amqp publish!")
		return
	}
	body, err1 := json.Marshal(data)
	if err1 != nil {
		a.logger.Error().Msg(err1.Error())
	}

	appId, appName := uint64(0), "unknown"
	if app != nil {
		appId = app.ID
		appName = app.Name
	}
	key := helpers.Concat("query.", strconv.FormatUint(chain.ID, 10), ".", strconv.FormatUint(appId, 10))
	err2 := a.amqp.Channel.Publish(a.config.String("amqp.exchange", "web3rpcproxy.query.topic"), key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err2 != nil {
		a.logger.Error().Msg(err2.Error())
	} else {
		utils.TotalAmqpMessages.WithLabelValues(chain.Code, appName).Inc()
	}
}
