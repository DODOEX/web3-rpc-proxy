package service

import (
	"context"
	"fmt"
	"math"
	reflect "reflect"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/endpoint"
	web3rpcprovider "github.com/DODOEX/web3rpcproxy/providers/web3-rpc-provider"
	"github.com/DODOEX/web3rpcproxy/utils"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type EndpointService interface {
	Init()
	Chains() []uint64
	GetAll(chain uint64) ([]*endpoint.Endpoint, bool)
	Purge()
}

type endpointService struct {
	logger   zerolog.Logger
	config   *config.Conf
	registry *prometheus.Registry
	cache    *endpoint.Cache
	provider *web3rpcprovider.Web3RPCProvider
}

func NewEndpointService(logger zerolog.Logger, config *config.Conf, provider *web3rpcprovider.Web3RPCProvider) EndpointService {
	service := &endpointService{
		logger:   logger.With().Str("name", "endpoint_service").Logger(),
		cache:    endpoint.NewCache(),
		config:   config,
		registry: prometheus.DefaultRegisterer.(*prometheus.Registry),
		provider: provider,
	}

	service.registry.MustRegister(utils.EndpointDurationSummary)
	service.registry.MustRegister(utils.EndpointStatusSummary)

	return service
}

func (s *endpointService) Init() {
	d := s.config.Duration("endpoints-refresh-interval", 30*time.Second)
	ticker2 := time.NewTicker(d)
	go func() {
		for t := range ticker2.C {
			err := s.refresh(d)
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to refresh endpoints")
			}
			s.logger.Debug().Msgf("Update endpoints At %v", t.Format(time.RFC3339Nano))
		}
	}()
}

func (s *endpointService) Chains() []uint64 {
	return s.cache.Chains()
}

func (s *endpointService) GetAll(chain uint64) ([]*endpoint.Endpoint, bool) {
	v, ok := s.cache.GetAll(chain)
	if !ok {
		v = s.load(chain)
		for i := range v {
			s.cache.Put(v[i])
		}
	}
	return v, len(v) > 0
}

func (s *endpointService) Purge() {
	for _, v := range s.cache.Chains() {
		s.cache.Purge(v)
	}
}

func mergeEndpiont(a, b *endpoint.Endpoint) *endpoint.Endpoint {
	return endpoint.Merge(a, b)
}

func makeUniqueId(e *endpoint.Endpoint) string {
	return fmt.Sprintf("%d%s", e.ChainID(), e.Url())
}

func (s *endpointService) load(chain uint64) []*endpoint.Endpoint {
	list := make([][]*endpoint.Endpoint, 2)
	endpoints1 := loadEndpointFromConfig(s.config, chain)
	list[0] = endpoints1

	if s.provider != nil {
		err, endpoints2, ok := s.provider.Provide(context.Background(), chain)
		if err != nil {
			s.logger.Debug().Msgf("Load provider failed %v", err)
			return endpoints1
		}
		if !ok {
			s.logger.Debug().Msgf("Not found endpoints: %v", endpoints2)
			return endpoints1
		}
		list[1] = endpoints2
	}

	return helpers.MergeSlicesBy(mergeEndpiont, makeUniqueId, list...)
}

func (s *endpointService) refresh(d time.Duration) error {
	mfs, err := s.registry.Gather()
	if err != nil {
		return err
	}

	for _, mf := range mfs {
		if mf.GetName() == utils.EndpointDurationSummaryName {
			for _, m := range mf.GetMetric() {
				summary := m.GetSummary()
				e, ok := s.cache.Get(m.GetLabel()[1].GetValue())
				if !ok || e.ChainCode() != m.GetLabel()[0].GetValue() {
					continue
				}

				ops := []endpoint.Attributer{}
				quantiles := summary.GetQuantile()
				for _, q := range quantiles {
					v := 0.0
					if !math.IsNaN(q.GetValue()) {
						v = q.GetValue()
					}
					switch q.GetQuantile() {
					case 0.95:
						ops = append(ops, endpoint.WithAttr(endpoint.P95Duration, v))
						s.logger.Debug().Msgf("%s %s p95 duration: %f", e.ChainCode(), e.Url(), e.P95Duration())
					}
				}
				if len(ops) > 0 {
					ops = append(ops, endpoint.WithAttr(endpoint.LastUpdateTime, time.Now()))
					e.Update(ops...)
				}
			}
		}

		if mf.GetName() == utils.EndpointStatusSummaryName {
			for _, m := range mf.GetMetric() {
				summary := m.GetSummary()
				e, ok := s.cache.Get(m.GetLabel()[1].GetValue())
				if !ok || e.ChainCode() != m.GetLabel()[0].GetValue() {
					continue
				}

				ops := []endpoint.Attributer{}
				quantiles := summary.GetQuantile()
				for _, q := range quantiles {
					v := true
					if !(q.GetValue() > 100.0 && q.GetValue() < 300.0) || math.IsNaN(q.GetValue()) {
						v = false
					}
					switch q.GetQuantile() {
					case 0.95:
						ops = append(ops, endpoint.WithAttr(endpoint.P95Health, v))
						s.logger.Debug().Msgf("%s %s p95 status: %d", e.ChainCode(), e.Url(), int(q.GetValue()))
					}
				}
				if len(ops) > 0 {
					ops = append(ops, endpoint.WithAttr(endpoint.LastUpdateTime, time.Now()))
					e.Update(ops...)
				}
			}
		}
	}

	return nil
}

func loadEndpointFromConfig(config *config.Conf, chain uint64) []*endpoint.Endpoint {
	mapToStates := func(infos []*common.EndpointInfo, fn func(int, *endpoint.Endpoint)) []*endpoint.Endpoint {
		return slice.Compact(slice.Map(infos, func(i int, info *common.EndpointInfo) *endpoint.Endpoint {
			e, err := endpoint.NewWithInfo(infos[i])
			if err != nil {
				return nil
			}
			fn(i, e)
			return e
		}))
	}

	if v := config.Get(helpers.Concat("chains.", fmt.Sprint(chain))); v != nil {
		chain := v.(common.EndpointChain)
		if (chain.Services == nil) && (chain.Endpoints == nil) {
			return nil
		}

		if chain.Services != nil {
			val := reflect.ValueOf(chain.Services).Elem()
			for i := 0; i < val.NumField(); i++ {
				if g, ok := val.Field(i).Interface().(common.EndpointList); ok {
					return mapToStates(g.Endpoints, func(j int, e *endpoint.Endpoint) {
						e.Update(
							endpoint.WithAttr(endpoint.ChainId, chain.ChainID),
							endpoint.WithAttr(endpoint.ChainCode, chain.ChainCode),
							endpoint.WithAttr(endpoint.Type, val.Type().Field(i).Name),
						)
					})
				}
			}
		} else {
			return mapToStates(chain.Endpoints, func(j int, e *endpoint.Endpoint) {
				e.Update(endpoint.WithAttr(endpoint.ChainId, chain.ChainID))
			})
		}
	}

	return nil
}
