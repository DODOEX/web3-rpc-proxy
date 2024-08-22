package utils

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

var prefix = os.Getenv("WEB3RPCPROXY_METRICS_PREFIX")

// 总请求数
var TotalRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: prefix + "total_requests",
		Help: "Total number of requests processed",
	},
	[]string{"chain", "app", "status"},
)

// 总耗时
var RequestDurations = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    prefix + "request_durations",
		Help:    "Total seconds of durations for request",
		Buckets: []float64{0.02, 0.05, 0.08, 0.1, 0.25, 0.5, 0.85, 1, 2, 5, 10},
	},
	[]string{"chain", "app"},
)

// 节点请求数
var TotalEndpoints = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: prefix + "total_endpoints",
		Help: "Total number of endpoints processed",
	},
	[]string{"chain", "url", "status"},
)

// 节点耗时
var EndpointDurations = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    prefix + "endpoint_durations",
		Help:    "Total seconds of durations for the endpoint",
		Buckets: []float64{0.02, 0.05, 0.08, 0.1, 0.25, 0.5, 0.85, 1, 2, 5, 10},
	},
	[]string{"chain", "url"},
)

// 缓存数
var TotalCaches = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: prefix + "total_caches",
		Help: "Total number of requests cached",
	},
	[]string{"chain", "app", "method", "status"},
)

// 消息数
var TotalAmqpMessages = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: prefix + "total_amqp_messages",
		Help: "Total number of publish messaged",
	},
	[]string{"chain", "app"},
)

var EndpointDurationSummaryName = prefix + "endpoint_url_durations"
var EndpointDurationSummary = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Name:       EndpointDurationSummaryName,
		Help:       "Total seconds of durations for the endpoint url",
		Objectives: map[float64]float64{0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	},
	[]string{"chain", "url"},
)

var EndpointStatusSummaryName = prefix + "endpoint_url_status"
var EndpointStatusSummary = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Name:       EndpointStatusSummaryName,
		Help:       "Total number of status for the endpoint url",
		Objectives: map[float64]float64{0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	},
	[]string{"chain", "url"},
)

var EndpointGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: prefix + "endpoint_status",
		Help: "Real time status of the endpoint",
	},
	[]string{
		"chain",
		"url",
		"weight",
		"health",
		"blocknumber",
		"duration",
	},
)
