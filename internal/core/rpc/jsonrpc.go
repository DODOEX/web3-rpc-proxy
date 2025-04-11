package rpc

import (
	"errors"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
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
	JSONRPC_ERROR    JSONRPC_Type = "error"
	JSONRPC_REQUEST  JSONRPC_Type = "request"
	JSONRPC_RESPONSE JSONRPC_Type = "response"
)

type JSONRPCPayload interface {
	ID() string

	Version() JSONRPC_Version

	Type() JSONRPC_Type

	Map() map[string]any

	Clone(...map[string]any) JSONRPCPayload

	UnmarshalJSON(b []byte) error

	MarshalJSON() ([]byte, error)
}

type JSONRPCResulter interface {
	JSONRPCPayload

	Result() any

	Error() any
}

type JSONRPCer interface {
	JSONRPCPayload

	Method() string

	Params() []any

	// 根据jsonrpc的id, version构建结果
	MakeResult(value any, err any) JSONRPCResulter
}

func newJSONRPC(root *ast.Node) jsonrpc {
	return jsonrpc{raw: root, idxs: map[string]int{"id": -1, "jsonrpc": -1, "method": -1, "params": -1, "result": -1, "error": -1}}
}

func UnmarshalJSONRPCs(b []byte) (jsonrpcs []JSONRPCer, batch bool, err error) {
	// 先解析为AST节点
	root, err := sonic.GetWithOptions(b, ast.SearchOptions{
		ValidateJSON: true,
	})
	if err != nil {
		return nil, false, err
	}
	if !root.Exists() {
		return nil, false, errors.New("jsonrpc not exists")
	}
	if err = root.Load(); err != nil {
		return nil, false, err
	}

	// 判断是否是数组
	if root.TypeSafe() == ast.V_ARRAY {
		// 批量处理
		l, err := root.Len()
		if err != nil {
			return nil, false, err
		}
		var (
			jsonrpcs = make([]JSONRPCer, l)
			i        = 0
		)
		root.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			jsonrpcs[i] = newJSONRPC(node)
			i++
			return true
		})
		return jsonrpcs, true, nil
	}

	// 单个请求
	return []JSONRPCer{newJSONRPC(&root)}, false, nil
}

func UnmarshalJSONRPCResults(b []byte) (jsonrpcs []JSONRPCResulter, batch bool, err error) {
	// 先解析为AST节点
	root, err := sonic.GetWithOptions(b, ast.SearchOptions{
		ValidateJSON: true,
	})
	if err != nil {
		return nil, false, err
	}
	if !root.Exists() {
		return nil, false, errors.New("jsonrpc not exists")
	}
	if err = root.Load(); err != nil {
		return nil, false, err
	}

	// 判断是否是数组
	if root.TypeSafe() == ast.V_ARRAY {
		// 批量处理
		len, err := root.Len()
		if err != nil {
			return nil, false, err
		}
		var (
			jsonrpcs = make([]JSONRPCResulter, len)
			i        = 0
		)
		root.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			jsonrpcs[i] = newJSONRPC(node)
			i++
			return true
		})
		return jsonrpcs, true, nil
	}

	// 单个请求
	return []JSONRPCResulter{newJSONRPC(&root)}, false, nil
}

func NewJSONRPC(ms ...map[string]any) jsonrpc {
	var m map[string]any
	if len(ms) > 0 {
		m = ms[0]
	}

	pairs := make([]ast.Pair, len(m))
	i := 0
	for k, v := range m {
		pairs[i] = ast.NewPair(k, ast.NewAny(v))
		i++
	}

	node := ast.NewObject(pairs)
	return newJSONRPC(&node)
}

type jsonrpc struct {
	idxs map[string]int
	raw  *ast.Node // 替换原来的raw map[string]any
}

func (j jsonrpc) indexOrGet(key string) *ast.Node {
	if j.idxs[key] > -1 {
		return j.raw.Index(j.idxs[key])
	}
	node, idx := j.raw.IndexOrGetWithIdx(j.idxs[key], key)
	if node != nil {
		j.idxs[key] = idx
	}
	return node
}

func (j jsonrpc) ID() string {
	node := j.indexOrGet("id")
	if node != nil {
		if v, err := node.String(); err == nil {
			return v
		}
	}
	return ""
}

func (j jsonrpc) Version() JSONRPC_Version {
	node := j.indexOrGet("jsonrpc")
	if node != nil {
		if v, err := node.String(); err == nil {
			return v
		}
	}
	return JSONRPC_VERSION_1
}

func (j jsonrpc) Method() string {
	node := j.indexOrGet("method")
	if node != nil {
		if v, err := node.String(); err == nil {
			return v
		}
	}
	return ""
}

func (j jsonrpc) Params() []any {
	node := j.indexOrGet("params")
	if node != nil {
		if node.TypeSafe() == ast.V_ARRAY {
			if v, err := node.Array(); err == nil {
				return v
			}
		} else {
			if v, err := node.Interface(); err == nil {
				return []any{v}
			}
		}
	}
	return []any{}
}

func (j jsonrpc) Result() any {
	node := j.indexOrGet("result")
	if node != nil {
		v, err := node.Interface()
		if err == nil {
			return v
		}
	}
	return nil
}

func (j jsonrpc) Error() any {
	node := j.indexOrGet("error")
	if node != nil {
		v, err := node.Interface()
		if err == nil {
			return v
		}
	}
	return nil
}

func (j jsonrpc) Type() JSONRPC_Type {
	id := j.indexOrGet("id")
	result := j.indexOrGet("result")
	error := j.indexOrGet("error")

	// check != nil
	if error != nil && error.TypeSafe() > ast.V_NULL {
		return JSONRPC_ERROR
	} else if j.Version() == JSONRPC_VERSION_2 && id != nil && id.TypeSafe() == ast.V_NULL {
		return JSONRPC_NOTIFY
	} else if result != nil && result.TypeSafe() > ast.V_NULL {
		return JSONRPC_RESPONSE
	}

	return JSONRPC_REQUEST
}

func (j jsonrpc) Map() map[string]any {
	if v, err := j.raw.Map(); err == nil {
		return v
	}
	return nil
}

func (j jsonrpc) Clone(ms ...map[string]any) JSONRPCPayload {
	var m map[string]any
	if len(ms) > 0 {
		m = ms[0]
	}

	if v, err := j.raw.Raw(); err == nil {
		if node, err := sonic.GetFromString(v); err == nil {
			for k, v := range m {
				node.Set(k, ast.NewAny(v))
			}
			return newJSONRPC(&node)
		}
	}
	return nil
}

func (j jsonrpc) UnmarshalJSON(b []byte) error {
	return j.raw.UnmarshalJSON(b)
}

func (j jsonrpc) MarshalJSON() ([]byte, error) {
	return j.raw.MarshalJSON()
}

func (j jsonrpc) MakeResult(value any, error any) JSONRPCResulter {
	node := ast.NewObject([]ast.Pair{})

	if id := j.indexOrGet("id"); id != nil {
		if v, err := id.Raw(); err == nil {
			node.Set("id", ast.NewRaw(v))
		}
	}
	if version := j.indexOrGet("jsonrpc"); version != nil {
		if v, err := version.Raw(); err == nil {
			node.Set("jsonrpc", ast.NewRaw(v))
		}
	}
	if value != nil {
		node.SetAny("result", value)
	}
	if error != nil {
		node.SetAny("error", error)
	}

	return newJSONRPC(&node)
}
