package rpc

import (
	"encoding/json"
	"strconv"

	"github.com/duke-git/lancet/v2/slice"
)

type JSONRPC_Version = string

const (
	JSONRPC_VERSION_0 JSONRPC_Version = ""
	JSONRPC_VERSION_1 JSONRPC_Version = "1.0"
	JSONRPC_VERSION_2 JSONRPC_Version = "2.0"
)

type JSONRPC_Type = string

const (
	JSONRPC_NOTIFY   JSONRPC_Type = "notify"
	JSONRPC_REQUEST  JSONRPC_Type = "request"
	JSONRPC_RESPONSE JSONRPC_Type = "response"
	JSONRPC_ERROR    JSONRPC_Type = "error"
)

type JSONRPCer interface {
	ID() string
	Version() JSONRPC_Version
	Method() string
	Params() []any

	Type() JSONRPC_Type

	Raw() map[string]any

	Seal() SealedJSONRPC
	// 根据jsonrpc的id, version构建结果
	MakeResult(value any, err any) SealedJSONRPCResult
}

func UnmarshalJSONRPCs(b []byte) (jsonrpcs []JSONRPCer, batch bool, err error) {
	var raws []map[string]any
	if err := json.Unmarshal(b, &raws); err != nil {
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, false, err
		}

		return []JSONRPCer{NewJSONRPC(raw)}, false, nil
	}

	return slice.Map(raws, func(i int, raw map[string]any) JSONRPCer {
		return NewJSONRPC(raw)
	}), true, nil
}

// 输出给端点的格式
type SealedJSONRPC struct {
	Params  []any  `json:"params"`
	ID      string `json:"id"`
	Version string `json:"jsonrpc,omitempty"`
	Method  string `json:"method"`
}

// 原始的请求
type jsonrpc struct {
	raw map[string]any
}

func NewJSONRPC(raw map[string]any) JSONRPCer {
	return jsonrpc{raw: raw}
}

func (req jsonrpc) RawID() any {
	return req.raw["id"]
}

func (req jsonrpc) RawVersion() string {
	if version, ok := req.raw["jsonrpc"].(string); ok {
		return version
	}
	return ""
}

func (req jsonrpc) RawMethod() string {
	if method, ok := req.raw["method"].(string); ok {
		return method
	}
	return ""
}

func (req jsonrpc) RawParams() any {
	return req.raw["params"]
}

func (req jsonrpc) ID() string {
	if id, ok := req.raw["id"].(float64); ok {
		_id := strconv.FormatInt(int64(id), 16)
		return _id
	} else if req.raw["id"] != nil {
		_id := req.raw["id"].(string)
		return _id
	}
	return ""
}

func (req jsonrpc) Version() JSONRPC_Version {
	if version, ok := req.raw["jsonrpc"].(string); ok {
		return version
	}
	return JSONRPC_VERSION_1
}

func (req jsonrpc) Method() string {
	if method, ok := req.raw["method"].(string); ok {
		return method
	}
	return ""
}

func (req jsonrpc) Params() []any {
	if req.raw["params"] != nil {
		if params, ok := req.raw["params"].([]any); ok {
			return params
		} else {
			return []any{req.raw["params"]}
		}
	}
	return []any{}
}

func (req jsonrpc) Type() JSONRPC_Type {
	if req.Version() == JSONRPC_VERSION_2 && req.raw["id"] == nil {
		return JSONRPC_NOTIFY
	}
	return JSONRPC_REQUEST
}

func (req jsonrpc) Raw() map[string]any {
	return req.raw
}

func (req jsonrpc) Seal() SealedJSONRPC {
	jsonrpc := SealedJSONRPC{
		ID:      req.ID(),
		Version: req.Version(),
		Method:  req.Method(),
		Params:  req.Params(),
	}
	return jsonrpc
}

func (req jsonrpc) MakeResult(value any, err any) SealedJSONRPCResult {
	return SealedJSONRPCResult{
		ID:      req.raw["id"],
		Version: req.raw["jsonrpc"].(string),
		Result:  value,
		Error:   err,
	}
}

func (req jsonrpc) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &req.raw); err != nil {
		return err
	}
	return nil
}

func (req jsonrpc) MarshalJSON() ([]byte, error) {
	return json.Marshal(req.Seal())
}

type JSONRPCResulter interface {
	ID() string
	Version() JSONRPC_Version
	Result() any
	Error() any

	Raw() map[string]any

	Type() JSONRPC_Type
}

func UnmarshalJSONRPCResults(b []byte) (results []JSONRPCResulter, batch bool, err error) {
	var raws []map[string]any
	if err := json.Unmarshal(b, &raws); err != nil {
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, false, err
		}

		return []JSONRPCResulter{NewJSONRPCResult(raw)}, false, nil
	}

	return slice.Map(raws, func(i int, raw map[string]any) JSONRPCResulter {
		return NewJSONRPCResult(raw)
	}), true, nil
}

// 输出给客户端的格式
type SealedJSONRPCResult struct {
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	Version string `json:"jsonrpc,omitempty"`
}

func (res SealedJSONRPCResult) MarshalJSON() ([]byte, error) {
	if res.Error != nil {
		return json.Marshal(struct {
			ID      any    `json:"id"`
			Result  any    `json:"result,omitempty"`
			Error   any    `json:"error"`
			Version string `json:"jsonrpc,omitempty"`
		}{
			ID:      res.ID,
			Version: res.Version,
			Result:  res.Result,
			Error:   res.Error,
		})
	}
	return json.Marshal(struct {
		ID      any    `json:"id"`
		Result  any    `json:"result"`
		Error   any    `json:"error,omitempty"`
		Version string `json:"jsonrpc,omitempty"`
	}{
		ID:      res.ID,
		Version: res.Version,
		Result:  res.Result,
		Error:   res.Error,
	})
}

type jsonrpc_result struct {
	raw map[string]any
}

func NewJSONRPCResult(raw map[string]any) JSONRPCResulter {
	return jsonrpc_result{raw: raw}
}

func (res jsonrpc_result) ID() string {
	if id, ok := res.raw["id"].(float64); ok {
		_id := strconv.FormatInt(int64(id), 16)
		return _id
	} else if res.raw["id"] != nil {
		_id := res.raw["id"].(string)
		return _id
	}
	return ""
}
func (res jsonrpc_result) Version() JSONRPC_Version {
	if version, ok := res.raw["jsonrpc"].(string); ok {
		return version
	}
	return JSONRPC_VERSION_1
}
func (res jsonrpc_result) Result() any {
	return res.raw["result"]
}

func (res jsonrpc_result) Error() any {
	return res.raw["error"]
}

func (res jsonrpc_result) Type() JSONRPC_Type {
	if res.Error() != nil {
		return JSONRPC_ERROR
	} else if res.Version() == JSONRPC_VERSION_2 && res.raw["id"] == nil {
		return JSONRPC_NOTIFY
	}
	return JSONRPC_RESPONSE
}

func (res jsonrpc_result) Raw() map[string]any {
	return res.raw
}

func (res jsonrpc_result) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &res.raw); err != nil {
		return err
	}
	return nil
}

func (res jsonrpc_result) MarshalJSON() ([]byte, error) {
	result := SealedJSONRPCResult{
		ID:      res.ID(),
		Version: res.Version(),
		Result:  res.Result(),
		Error:   res.Error(),
	}
	return json.Marshal(result)
}

func MarshalJSONRPCResults(data any) ([]byte, error) {
	if v, ok := data.([]SealedJSONRPCResult); ok {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		return b, nil
	}

	if v, ok := data.(SealedJSONRPCResult); ok {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		return b, nil
	}

	return nil, nil
}
