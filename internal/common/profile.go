package common

import "github.com/DODOEX/web3rpcproxy/utils/general/names"

type QueryStatus string

const (
	Success   QueryStatus = "success"   // 接受请求，请求成功
	Fail      QueryStatus = "fail"      // 接受请求，请求失败
	Timeout   QueryStatus = "timeout"   // 接受请求，但请求超时
	Intercept QueryStatus = "intercept" // 接受请求，但请求被拦截
	Reject    QueryStatus = "reject"    // 拒绝请求，e.g.: 限流，黑名单
	Error     QueryStatus = "error"     // 内部报错
)

type OptionsProfile = struct {
	SpecifiedUpstreamTypes []string      `json:"specifiedUpstreamTypes,omitempty"`
	ForceUpstreamType      string        `json:"forceUpstreamType,omitempty"`
	Timeout                names.Seconds `json:"timeout,omitempty"`

	BeforeBlocksUseScanApi int `json:"beforeBlocksUseScanApi,omitempty"`
	BeforeBlocksUseActive  int `json:"beforeBlocksUseActive,omitempty"`

	MaxRetryCount int  `json:"maxRetryCount,omitempty"`
	UseCache      bool `json:"useCache,omitempty"`
	UseScanApi    bool `json:"useScanApi,omitempty"`

	EthCallUseFullNode bool `json:"ethCallUseFullNode,omitempty"`
}

type RequestProfile = struct {
	Methods   []string           `json:"methods"`
	ReqID     names.UUIDv4       `json:"reqId"` // 转发服务器，内部代理的请求id
	Url       string             `json:"url"`
	Timestamp names.Milliseconds `json:"timestamp"`
}

type ResponseProfile = struct {
	ReqID    names.UUIDv4       `json:"reqId"`
	Error    string             `json:"error,omitempty"`
	Message  string             `json:"message,omitempty"`
	Code     string             `json:"code,omitempty"`
	Duration names.Milliseconds `json:"duration"`
	Traffic  names.Bytes        `json:"traffic"`
	Status   int                `json:"status"`
	Respond  bool               `json:"respond"` // 是否作为最终结果返回给客户端
}

// 用户的单个请求，batchcall 以数组表示
type QueryProfile = struct {
	Options OptionsProfile `json:"options"`

	// 代理发起的请求, 重试会产生多个请求
	Requests []RequestProfile `json:"requests"`

	// 代理接收到的结果，重试会产生多个
	Responses []ResponseProfile `json:"responses"`

	// 原始请求状态
	ID        names.UUIDv4 `json:"id"`
	Href      names.Url    `json:"href"`   // 用户完整的请求URL
	Method    string       `json:"method"` // 用户请求的方法
	IP        string       `json:"ip"`     // 用户的ip
	IPCountry string       `json:"ipCountry"`

	Status QueryStatus `json:"status"`

	// 请求相关参数
	AppID   uint64 `json:"appId"`
	ChainID uint64 `json:"chainId"`

	// 代理请求的结果
	Starttime names.Milliseconds `json:"startTime"` // 接受请求的时间戳
	Endtime   names.Milliseconds `json:"endTime"`   // 请求结束的时间戳
}
