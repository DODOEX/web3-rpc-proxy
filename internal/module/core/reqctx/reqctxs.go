package reqctx

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/shared"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/google/uuid"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type Reqctxs interface {
	Logger() *zerolog.Logger
	ReqID() string
	ChainID() common.ChainId
	Body() *[]byte
	Options() Options
	Config() *config.Conf
	QueryArgs() *fasthttp.Args
	AppKey() string
	AppBucket() string
	App() *common.App
	SetApp(app *common.App)
	Profile() *common.QueryProfile
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

type reqctx struct {
	logger     zerolog.Logger
	chain      *common.Chain
	requestCtx *fasthttp.RequestCtx
	app        *common.App
	config     *config.Conf
	options    Options
	profile    *common.QueryProfile
	uuid       string
}

func NewReqctx(requestCtx *fasthttp.RequestCtx, cfg *config.Conf, logger zerolog.Logger) Reqctxs {
	rc := &reqctx{
		requestCtx: requestCtx,
		config:     cfg,
		logger:     logger.With().Str("name", "reqctx").Logger(),
		profile: &common.QueryProfile{
			Starttime: time.Now().UnixMilli(),
			Requests:  []common.RequestProfile{},
			Responses: []common.ResponseProfile{},
		},
	}

	rc.logger = rc.logger.With().Str("uuid", rc.ReqID()).Str("chain", fmt.Sprint(rc.ChainID())).Logger()

	rc.profile.ID = rc.ReqID()
	rc.profile.Method = string(requestCtx.Method())
	rc.profile.Href = string(requestCtx.RequestURI())
	if v := requestCtx.Request.Header.Peek("cf-connecting-ip"); len(v) > 0 {
		rc.profile.IP = string(v)
	} else if v := requestCtx.Request.Header.Peek("true-client-ip"); len(v) > 0 {
		rc.profile.IP = string(v)
	} else {
		rc.profile.IP = requestCtx.RemoteIP().String()
	}
	if v := requestCtx.Request.Header.Peek("cf-ipcountry"); len(v) > 0 {
		rc.profile.IPCountry = string(v)
	}
	rc.profile.ChainID = rc.ChainID()

	return rc
}

func (c *reqctx) App() *common.App {
	return c.app
}

func (c *reqctx) SetApp(app *common.App) {
	c.app = app
	c.profile.AppID = c.app.ID
	// 将租户的配置加载到 koanf
	value := app.Preferences.Get()
	if data, ok := value.(map[string]any); ok && data["__configuration"] != nil {
		if err := c.config.Load(confmap.Provider(data["__configuration"].(map[string]any), "."), nil); err != nil {
			c.logger.Error().Msgf("error loading default values: %v", err)
		} else {
			config.LoadEndpointChains(c.config, shared.KoanfEndpointsToken)
		}
	}
}

func (c *reqctx) Logger() *zerolog.Logger {
	return &c.logger
}

func (c *reqctx) AppKey() string {
	if v := c.requestCtx.UserValue("apikey"); v != nil {
		return v.(string)
	} else if v := c.requestCtx.Request.Header.Peek("x-api-key"); len(v) > 0 {
		return string(v)
	} else if c.requestCtx.Request.URI().QueryArgs().Has("x_api_key") {
		return string(c.requestCtx.Request.URI().QueryArgs().Peek("x_api_key"))
	}
	return ""
}

func (c *reqctx) AppBucket() string {
	if v := c.requestCtx.Request.Header.Peek("x-api-bucket"); len(v) > 0 {
		return string(v)
	} else if c.requestCtx.Request.URI().QueryArgs().Has("x_api_bucket") {
		return string(c.requestCtx.Request.URI().QueryArgs().Peek("x_api_bucket"))
	}
	return "default"
}

func (c *reqctx) QueryArgs() *fasthttp.Args {
	return c.requestCtx.Request.URI().QueryArgs()
}

func (c *reqctx) Config() *config.Conf {
	return c.config
}

func (c *reqctx) Options() Options {
	if c.options == nil {
		options := NewOptions(c, c.app)
		c.options = options
		c.profile.Options = c.Options().ToProfile()
	}
	return c.options
}

func (c *reqctx) ReqID() string {
	if c.uuid == "" {
		if v := c.requestCtx.Request.Header.Peek("x-req-id"); len(v) > 0 {
			c.uuid = string(v)
		} else if v := c.requestCtx.Request.Header.Peek("x-request-id"); len(v) > 0 {
			c.uuid = string(v)
		} else {
			c.uuid = uuid.NewString()
		}
	}
	return c.uuid
}

func (c *reqctx) ChainID() common.ChainId {
	if c.chain == nil {
		if v := c.Config().Get(helpers.Concat("chains.", c.requestCtx.UserValue("chain").(string))); v != nil {
			chain := v.(common.EndpointChain)
			c.chain = &common.Chain{
				ID:   chain.ChainID,
				Code: chain.ChainCode,
			}
		} else {
			c.chain = &common.Chain{}
			if v, err := strconv.ParseUint(c.requestCtx.UserValue("chain").(string), 10, 64); err == nil {
				c.chain.ID = v
			}
		}
	}

	return c.chain.ID
}

func (c *reqctx) Body() *[]byte {
	body := c.requestCtx.PostBody()
	return &body
}

func (c *reqctx) Profile() *common.QueryProfile {
	return c.profile
}

func (c *reqctx) Deadline() (deadline time.Time, ok bool) {
	return c.requestCtx.Deadline()

}
func (c *reqctx) Done() <-chan struct{} {
	return c.requestCtx.Done()

}
func (c *reqctx) Err() error {
	return c.requestCtx.Err()

}
func (c *reqctx) Value(key any) any {
	return c.requestCtx.Value(key)
}

type RetryStrategy int8

const (
	Same     RetryStrategy = 0 // 总是重试相同的节点
	Rotation RetryStrategy = 1 // 交替重试，可用节点
)

func (s RetryStrategy) String() string {
	switch s {
	case Same:
		return "same"
	case Rotation:
		return "rotation"
	default:
		return "unknown"
	}
}

func ParseRetryStrategy(s string) RetryStrategy {
	switch s {
	case "same":
		fallthrough
	case "Same":
		return Same
	case "rotation":
		fallthrough
	case "Rotation":
		fallthrough
	default:
		return Rotation
	}
}

type EndpointType = string

const EndpointType_Default EndpointType = "default"

const (
	MaxAttempts = 30
	MaxTimeout  = time.Duration(5) * time.Minute
)

type Options interface {
	AgreeConverging() bool
	AgreeMultiCall() bool
	AllowChainIDs() []string
	AllowMethods() []string
	AllowContractAddresses() []string
	Caches() bool
	Attempts() int
	Timeout() time.Duration
	Secret() (*string, error)
	EndpointTypes() []EndpointType
	AttemptStrategy() RetryStrategy
	ToProfile() common.OptionsProfile
}

func MakeOptionsFeature(o Options) string {
	strategy, types := fmt.Sprint(o.AttemptStrategy()), o.EndpointTypes()
	slice.Sort(types)
	txt := strings.Join(slice.Concat([]string{strategy}, types), ",")
	return helpers.Short(txt)
}

type Option struct {
	reqctx Reqctxs
	app    *common.App
}

func NewOptions(reqctx Reqctxs, app *common.App) Options {
	return &Option{
		reqctx: reqctx,
		app:    app,
	}
}

// 愿意与他人的请求合并成batch call
func (o *Option) AgreeConverging() bool {
	return false
}

// 愿意将请求转成合约multicall发出
func (o *Option) AgreeMultiCall() bool {
	return false
}
func (o *Option) AllowChainIDs() []string {
	return nil
}
func (o *Option) AllowMethods() []string {
	return nil
}
func (o *Option) AllowContractAddresses() []string {
	return nil
}

func (o *Option) Caches() bool {
	if o.reqctx.QueryArgs().Has("cache") {
		if cache, err := strconv.ParseBool(string(o.reqctx.QueryArgs().Peek("cache"))); err == nil {
			return cache
		}
	}
	if o.reqctx.QueryArgs().Has("useCache") {
		if cache, err := strconv.ParseBool(string(o.reqctx.QueryArgs().Peek("useCache"))); err == nil {
			return cache
		}
	}
	return true
}

func (o *Option) Attempts() int {
	if o.reqctx.QueryArgs().Has("attempts") {
		if v, err := strconv.Atoi(string(o.reqctx.QueryArgs().Peek("attempts"))); err == nil && v > 0 {
			return int(math.Min(float64(v), MaxAttempts))
		}
	}
	if o.reqctx.QueryArgs().Has("maxRetryCount") {
		if v, err := strconv.Atoi(string(o.reqctx.QueryArgs().Peek("maxRetryCount"))); err == nil && v > 0 {
			return int(math.Min(float64(v), MaxAttempts))
		}
	}
	return 3
}

func (o *Option) Timeout() time.Duration {
	if o.reqctx.QueryArgs().Has("timeout") {
		if v, err := strconv.Atoi(string(o.reqctx.QueryArgs().Peek("timeout"))); err == nil {
			// 超时时间不能超过5分钟
			if timeout := time.Duration(v) * time.Millisecond; timeout < MaxTimeout {
				return timeout
			} else {
				return MaxTimeout
			}
		}
	}
	return 30 * time.Second
}

func (o *Option) EndpointTypes() []EndpointType {
	if o.reqctx.QueryArgs().Has("endpoint_type") {
		types := strings.Split(string(o.reqctx.QueryArgs().Peek("endpoint_type")), ",")
		return types
	}
	if o.reqctx.QueryArgs().Has("forceUpstreamType") {
		_t := string(o.reqctx.QueryArgs().Peek("forceUpstreamType"))
		return []EndpointType{_t, EndpointType_Default}
	}
	if o.reqctx.QueryArgs().Has("specifiedUpstreamTypes") {
		types := strings.Split(string(o.reqctx.QueryArgs().Peek("specifiedUpstreamTypes")), ",")
		return types
	}

	return []EndpointType{EndpointType_Default}
}

func (o *Option) Secret() (*string, error) {
	return nil, nil
}

func (o *Option) AttemptStrategy() RetryStrategy {
	if o.reqctx.QueryArgs().Has("attempt_strategy") {
		t := string(o.reqctx.QueryArgs().Peek("attempt_strategy"))
		return ParseRetryStrategy(t)
	}
	return Rotation
}

func (o *Option) ToProfile() common.OptionsProfile {
	beforeBlocksUseScanApi, _ := strconv.Atoi(string(o.reqctx.QueryArgs().Peek("beforeBlocksUseScanApi")))
	beforeBlocksUseActive, _ := strconv.Atoi(string(o.reqctx.QueryArgs().Peek("beforeBlocksUseActive")))
	return common.OptionsProfile{
		Timeout:                float64(o.Timeout().Milliseconds()),
		UseCache:               o.Caches(),
		UseScanApi:             o.reqctx.QueryArgs().Has("useScanApi"),
		MaxRetryCount:          o.Attempts(),
		SpecifiedUpstreamTypes: strings.Split(string(o.reqctx.QueryArgs().Peek("specifiedUpstreamTypes")), ","),
		ForceUpstreamType:      string(o.reqctx.QueryArgs().Peek("forceUpstreamType")),
		EthCallUseFullNode:     o.reqctx.QueryArgs().Has("ethCallUseFullNode"),
		BeforeBlocksUseScanApi: beforeBlocksUseScanApi,
		BeforeBlocksUseActive:  beforeBlocksUseActive,
	}
}
