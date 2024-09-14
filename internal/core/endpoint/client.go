package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/core/rpc"
	"github.com/DODOEX/web3rpcproxy/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type Client interface {
	Call(ctx context.Context, data []rpc.SealedJSONRPC, profiles ...*common.ResponseProfile) (result []rpc.JSONRPCResulter, err error)
	Close() error
}

type ClientFactoryConfig struct {
	Transport     *http.Transport
	JSONRPCSchema *rpc.JSONRPCSchema
	ClientsSize   int
}

type ClientFactory struct {
	cache  *lru.Cache[string, Client]
	config *ClientFactoryConfig
}

func NewClientFactory(config *ClientFactoryConfig) *ClientFactory {
	v, err := lru.NewWithEvict(config.ClientsSize, func(_ string, val Client) {
		val.Close()
	})
	if err != nil {
		log.Panicf("lru.New error: %v", err)
	}
	return &ClientFactory{
		cache:  v,
		config: config,
	}
}

func (ef *ClientFactory) GetClient(endpoint *Endpoint) Client {
	url := endpoint.Url().String()
	if client, ok := ef.cache.Get(url); ok {
		return client
	}

	var client Client
	if strings.HasPrefix(url, "wss://") || strings.HasPrefix(url, "ws://") {
		for i := 0; i < 3; i++ {
			client = NewWebSocketClient(endpoint, &websocketClientConfig{
				Transport:     ef.config.Transport,
				JSONRPCSchema: ef.config.JSONRPCSchema,
			})
			if client != nil {
				break
			}
		}
	} else {
		client = NewHTTPClient(endpoint, &httpClientConfig{
			Transport:     ef.config.Transport,
			JSONRPCSchema: ef.config.JSONRPCSchema,
		})
	}

	if client != nil {
		ef.cache.Add(url, client)
	}
	return client
}

func (ef *ClientFactory) Clear() {
	ef.cache.Purge()
}

func _EndpointGauge(e *Endpoint) prometheus.Gauge {
	var h string = "0"
	if e.Health() {
		h = "1"
	}
	return utils.EndpointGauge.WithLabelValues(
		e.ChainCode(),
		e.Url().String(),
		strconv.Itoa(e.Weight()),
		h,
		strconv.FormatUint(e.BlockNumber(), 10),
		strconv.FormatFloat(e.Duration(), 'f', 0, 64),
	)
}

func updateMetrics(endpoint *Endpoint, profile *common.ResponseProfile) {
	ops := []Attributer{
		WithAttrIncrease(Count, 1),
		WithAttr(LastUpdateTime, time.Now()),
	}

	if profile.Duration > 0 {
		ops = append(ops, WithAttr(Duration, profile.Duration*1.0))
	}
	if profile.Code == "" && profile.Status >= 200 && profile.Status < 300 {
		ops = append(ops, WithAttr(Health, true))
	} else {
		ops = append(ops, WithAttr(Health, false))
	}

	endpoint.Update(ops...)
}

func validateResults(logger zerolog.Logger, jrpcSchema *rpc.JSONRPCSchema, profile *common.ResponseProfile, data []rpc.SealedJSONRPC, results []rpc.JSONRPCResulter) error {
	for i := range results {
		if err := jrpcSchema.ValidateResponse(data[i].Method, results[i].Raw(), true); err != nil {
			v1, _ := json.Marshal(data[i])
			v2, _ := json.Marshal(results[i].Raw())
			logger.Warn().Msgf("Failed to validate %s / %s", v1, v2)
			profile.Code = "schema_validation_failed"
			profile.Message = string(v2)
			profile.Error = err.Error()
			return common.UpstreamServerError("Validating response failed", err)
		}
	}

	return nil
}

func recordingErrorResult(profile *common.ResponseProfile, result rpc.JSONRPCResulter) {
	if v, ok := result.Error().(map[string]any); ok {
		profile.Code = fmt.Sprint(v["code"])
		profile.Message = fmt.Sprint(v["message"])
	} else {
		profile.Code = "unknown_error"
		profile.Message = fmt.Sprint(v)
	}
}
