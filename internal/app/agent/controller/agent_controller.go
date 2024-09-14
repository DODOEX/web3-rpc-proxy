package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/app/agent/service"
	"github.com/DODOEX/web3rpcproxy/internal/app/shared"
	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
	"github.com/DODOEX/web3rpcproxy/internal/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/rs/zerolog"
	"github.com/streadway/amqp"
	"github.com/valyala/fasthttp"
)

type agentControllerConfig struct {
	AppName             string
	EnableTenantFeature bool
	AmqpExchange        string
}

type agentController struct {
	logger          zerolog.Logger
	conf            *config.Conf
	amqp            *shared.Amqp
	agentService    service.AgentService
	tenantService   service.TenantService
	endpointService service.EndpointService
	config          agentControllerConfig
}

type AgentController interface {
	HandleCall(ctx *fasthttp.RequestCtx)
}

func NewAgentController(
	logger zerolog.Logger,
	conf *config.Conf,
	amqp *shared.Amqp,
	agentService service.AgentService,
	tenantService service.TenantService,
	endpointService service.EndpointService,
) AgentController {
	controller := &agentController{
		conf:            conf,
		amqp:            amqp,
		logger:          logger.With().Str("name", "agent_controller").Logger(),
		agentService:    agentService,
		tenantService:   tenantService,
		endpointService: endpointService,
		config: agentControllerConfig{
			AppName:             conf.String("app.name", "Web3 RPC Proxy"),
			EnableTenantFeature: conf.Bool("tenant.enable", false),
			AmqpExchange:        conf.String("amqp.exchange", "web3rpcproxy.query.topic"),
		},
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
		rc         = a.getRequestContext(ctx)
		chainId    = rc.ChainID()
	)

	if endpoints, ok := a.endpointService.GetAll(chainId); !ok || len(endpoints) <= 0 {
		// 暂不支持该链
		rc.Logger().Warn().Msgf("Unsupport chain: %s", fmt.Sprint(ctx.UserValue("chain")))
		err := common.NotFoundError("Unsupported")

		status = err.QueryStatus()
		statusCode = err.StatusCode()
		body = err.Body()
	} else {
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
			ctx.Response.Header.Set("Server", a.config.AppName)
			ctx.Response.Header.SetContentType("application/json; charset=utf-8")
			ctx.SetBody(body)
			ctx.SetStatusCode(statusCode)
		}
	}()

	app, p := rc.App(), rc.Profile()
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
	if a.amqp.Conn != nil && chainId != 0 && p != nil {
		go a.publish(chainId, app, p)
	}

	appName := "unknown"
	if app != nil {
		appName = app.Name
	}
	utils.TotalRequests.WithLabelValues(fmt.Sprint(chainId), appName, string(status)).Inc()
	utils.RequestDurations.WithLabelValues(fmt.Sprint(chainId), appName).Observe(float64(p.Endtime-p.Starttime) / 1000.0)
	rc.Logger().Info().Any("status", status).TimeDiff("ms", time.UnixMilli(p.Endtime), time.UnixMilli(p.Starttime)).Msgf("%s %s %d", ctx.Method(), ctx.RequestURI(), statusCode)
}

func (a agentController) call(rc reqctx.Reqctxs, endpoints []*endpoint.Endpoint) ([]byte, common.HTTPErrors) {
	ctx, cancel := context.WithTimeoutCause(rc, rc.Options().Timeout(), common.TimeoutError("Request timed out"))
	defer cancel()

	// 解析 app
	if a.config.EnableTenantFeature && rc.App() == nil {
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
	return reqctx.NewReqctx(ctx, a.conf.Copy(), a.logger)
}

func (a agentController) publish(chainId common.ChainId, app *common.App, data *common.QueryProfile) {
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
	key := helpers.Concat("query.", strconv.FormatUint(chainId, 10), ".", strconv.FormatUint(appId, 10))
	err2 := a.amqp.Channel.Publish(a.config.AmqpExchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err2 != nil {
		a.logger.Error().Msg(err2.Error())
	} else {
		utils.TotalAmqpMessages.WithLabelValues(fmt.Sprint(chainId), appName).Inc()
	}
}
