package endpoint

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/reqctx"
	"github.com/DODOEX/web3rpcproxy/utils"
)

type Endpoint struct {
	state map[string]any
	rwm   sync.RWMutex
}

func New(url *url.URL) (e *Endpoint) {
	e = &Endpoint{
		state: make(map[string]any),
	}
	e.state[Type] = reqctx.EndpointType_Default
	e.state[Url] = url
	e.state[Health] = true
	e.state[P95Health] = true
	e.state[Duration] = 0
	e.state[P95Duration] = 0
	e.state[Count] = 0
	e.state[LastUpdateTime] = time.Now()
	return
}

func NewWithInfo(info *common.EndpointInfo) (*Endpoint, error) {
	parsedURL, err := url.Parse(info.Url)
	if err != nil {
		return nil, err
	}
	e := New(parsedURL)
	if info.Headers != nil {
		e.state[Headers] = *info.Headers
	} else {
		e.state[Headers] = make(map[string]string)
	}
	if info.Weight != nil {
		e.state[Weight] = *info.Weight
	} else {
		e.state[Weight] = 0
	}
	return e, nil
}

type EndpointAttribute = string

const (
	ChainId        EndpointAttribute = "chain_id"
	ChainCode      EndpointAttribute = "chain_code"
	Type           EndpointAttribute = "type"
	Count          EndpointAttribute = "count"
	LastUpdateTime EndpointAttribute = "last_update_time"
	BlockNumber    EndpointAttribute = "block_number"
	Health         EndpointAttribute = "health"
	Duration       EndpointAttribute = "duration" // ms
	P95Health      EndpointAttribute = "p95_health"
	P95Duration    EndpointAttribute = "p95_duration"
	Url            EndpointAttribute = "url"
	Headers        EndpointAttribute = "headers"
	Weight         EndpointAttribute = "weight"
)

func (e *Endpoint) Read(name EndpointAttribute) any {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	return e.state[name]
}

func _int(v any) int {
	if _v, ok := v.(int); ok {
		return _v
	}
	return 0
}
func _uint64(v any) uint64 {
	if _v, ok := v.(uint64); ok {
		return _v
	}
	return 0
}
func _float64(v any) float64 {
	if _v, ok := v.(float64); ok {
		return _v
	}
	return 0.0
}
func _string(v any) string {
	if _v, ok := v.(string); ok {
		return _v
	}
	return ""
}
func _bool(v any) bool {
	if _v, ok := v.(bool); ok {
		return _v
	}
	return false
}
func _time(v any) time.Time {
	if _v, ok := v.(time.Time); ok {
		return _v
	}
	var t time.Time
	return t
}
func _map(v any) map[string]string {
	if _v, ok := v.(map[string]string); ok && _v != nil {
		return _v
	}
	return nil
}
func _ptr[T any](v any) T {
	if _v, ok := v.(*T); ok && _v != nil {
		return *_v
	}
	var t T
	return t
}

func (e *Endpoint) ChainID() uint64 {
	return _uint64(e.Read(ChainId))
}
func (e *Endpoint) ChainCode() string {
	return _string(e.Read(ChainCode))
}
func (e *Endpoint) Type() string {
	return _string(e.Read(Type))
}
func (e *Endpoint) Count() uint64 {
	return _uint64(e.Read(Count))
}
func (e *Endpoint) LastUpdateTime() time.Time {
	return _time(e.Read(LastUpdateTime))
}
func (e *Endpoint) BlockNumber() uint64 {
	return _uint64(e.Read(BlockNumber))
}
func (e *Endpoint) Health() bool {
	return _bool(e.Read(Health))
}
func (e *Endpoint) Duration() float64 {
	return _float64(e.Read(Duration))
}
func (e *Endpoint) P95Health() bool {
	return _bool(e.Read(P95Health))
}
func (e *Endpoint) P95Duration() float64 {
	return _float64(e.Read(P95Duration))
}
func (e *Endpoint) Url() *url.URL {
	if _v, ok := e.Read(Url).(*url.URL); ok && _v != nil {
		return _v
	}
	return nil
}
func (e *Endpoint) Headers() map[string]string {
	return _map(e.Read(Headers))
}
func (e *Endpoint) Weight() int {
	return _int(e.Read(Weight))
}
func (e *Endpoint) String() string {
	return fmt.Sprintf("[%d %s]", e.ChainID(), e.Url())
}
func (e *Endpoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ChainID uint64 `json:"chainId"`
		Url		string `json:"url"`
		Weight 	int `json:"weight"`
	}{
		ChainID: e.ChainID(),
		Url: e.Url().String(),
		Weight: e.Weight(),
	})
}

type Attributer interface {
	apply(e *Endpoint)
}

func (e *Endpoint) With(name EndpointAttribute, v any) Attributer {
	return attribute{name: name, value: v}
}

func (e *Endpoint) Update(attributes ...Attributer) {
	if e == nil {
		return
	}

	c, h, d := e.Count(), e.Health(), e.Duration()

	e.rwm.Lock()
	for i := 0; i < len(attributes); i++ {
		attributes[i].apply(e)
	}
	e.rwm.Unlock()

	if e.Count() != c || e.Health() != h || e.Duration() != d {
		lables := []string{e.ChainCode(), e.Url().String()}
		utils.EndpointDurationSummary.WithLabelValues(lables...).Observe(e.Duration())
		if e.Health() {
			utils.EndpointStatusSummary.WithLabelValues(lables...).Observe(200.0)
		} else {
			utils.EndpointStatusSummary.WithLabelValues(lables...).Observe(500.0)
		}
	}
}

type attribute struct {
	name      string
	increment bool
	value     any
}

func (a attribute) apply(e *Endpoint) {
	if a.increment && e.state[a.name] != nil && a.value != nil {
		switch e.state[a.name].(type) {
		case int:
			e.state[a.name] = e.state[a.name].(int) + a.value.(int)
		case int8:
			e.state[a.name] = e.state[a.name].(int8) + a.value.(int8)
		case int16:
			e.state[a.name] = e.state[a.name].(int16) + a.value.(int16)
		case int32:
			e.state[a.name] = e.state[a.name].(int32) + a.value.(int32)
		case int64:
			e.state[a.name] = e.state[a.name].(int64) + a.value.(int64)
		case uint:
			e.state[a.name] = e.state[a.name].(uint) + a.value.(uint)
		case uint8:
			e.state[a.name] = e.state[a.name].(uint8) + a.value.(uint8)
		case uint16:
			e.state[a.name] = e.state[a.name].(uint16) + a.value.(uint16)
		case uint32:
			e.state[a.name] = e.state[a.name].(uint32) + a.value.(uint32)
		case uint64:
			e.state[a.name] = e.state[a.name].(uint64) + a.value.(uint64)
		case float32:
			e.state[a.name] = e.state[a.name].(float32) + a.value.(float32)
		case float64:
			e.state[a.name] = e.state[a.name].(float64) + a.value.(float64)
		}
	} else {
		e.state[a.name] = a.value
	}
}

func WithAttr(name EndpointAttribute, v any) Attributer {
	return attribute{name: name, value: v}
}

func WithAttrIncrease(name EndpointAttribute, v any) Attributer {
	return attribute{name: name, value: v, increment: true}
}
