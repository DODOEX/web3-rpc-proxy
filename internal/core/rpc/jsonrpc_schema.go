package rpc

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"encoding/json"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/xeipuuv/gojsonschema"
)

type JSONRPCSchema struct {
	requestSchemas   map[string]*gojsonschema.Schema
	_requestSchemas  map[string]map[string]any
	responseSchemas  map[string]*gojsonschema.Schema
	_responseSchemas map[string]map[string]any
}

// OpenRPC schema structures
type OpenRPCSchema struct {
	Info    Info     `json:"info"`
	Methods []Method `json:"methods"`
	OpenRPC string   `json:"openrpc"`
}

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type Method struct {
	Result Result  `json:"result"`
	Params []Param `json:"params"`
	Name   string  `json:"name"`
}

type Param struct {
	Schema json.RawMessage `json:"schema"`
	Name   string          `json:"name"`
}

type Result struct {
	Schema json.RawMessage `json:"schema"`
	Name   string          `json:"name"`
}

func NewJSONRPCSchema(b []byte) *JSONRPCSchema {
	jrpcSchema := &JSONRPCSchema{
		requestSchemas:   make(map[string]*gojsonschema.Schema),
		_requestSchemas:  make(map[string]map[string]any),
		responseSchemas:  make(map[string]*gojsonschema.Schema),
		_responseSchemas: make(map[string]map[string]any),
	}

	if len(b) == 0 {
		return jrpcSchema
	}

	// 解析 OpenRPC 规范
	var openrpcSchema OpenRPCSchema
	if err := json.Unmarshal(b, &openrpcSchema); err != nil {
		log.Fatalf("Error parsing OpenRPC schema: %v", err)
	}

	// 为每个方法生成 JSON Schema
	for i := range openrpcSchema.Methods {
		// 创建请求 JSON Schema
		requestSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"jsonrpc": map[string]any{
					"type": "string",
					"enum": []string{"2.0"},
				},
				"method": map[string]any{
					"type": "string",
					"enum": []string{openrpcSchema.Methods[i].Name},
				},
				"params": map[string]any{
					"type":  "array",
					"items": openrpcSchema.Methods[i].Params,
				},
				"id": map[string]any{
					"type": []string{"integer", "string"},
				},
			},
			"required": []string{"jsonrpc", "method", "params", "id"},
		}

		// 创建错误对象的 JSON Schema
		errorSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code":    map[string]any{"type": "integer"},
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
			"required": []string{"code", "message"},
		}

		// 创建响应 JSON Schema
		responseSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"jsonrpc": map[string]any{
					"type": "string",
					"enum": []string{"2.0"},
				},
				"result": openrpcSchema.Methods[i].Result.Schema,
				"error":  errorSchema,
				"id": map[string]any{
					"type": []string{"integer", "string", "null"},
				},
			},
			"oneOf": []map[string]interface{}{
				{"required": []string{"jsonrpc", "id", "result"}},
				{"required": []string{"error"}},
			},
		}

		// 输出 JSON Schema
		jrpcSchema._requestSchemas[openrpcSchema.Methods[i].Name] = requestSchema
		if _schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(requestSchema)); err == nil {
			jrpcSchema.requestSchemas[openrpcSchema.Methods[i].Name] = _schema
		}
		jrpcSchema._responseSchemas[openrpcSchema.Methods[i].Name] = responseSchema
		if _schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(responseSchema)); err == nil {
			jrpcSchema.responseSchemas[openrpcSchema.Methods[i].Name] = _schema
		}
	}

	return jrpcSchema
}

func (s *JSONRPCSchema) ValidateRequest(method string, raw map[string]any) error {
	if s == nil {
		return nil
	}
	if s.requestSchemas[method] == nil {
		return nil
	}

	result, err := s.requestSchemas[method].Validate(gojsonschema.NewRawLoader(raw))
	if err != nil {
		return err
	}

	if !result.Valid() {
		descriptions := slice.Map(result.Errors(), func(i int, err gojsonschema.ResultError) string {
			return "'" + err.Field() + "' " + err.Description()
		})
		return errors.New(method + ": " + slice.Join(descriptions, "; ") + ". please read the schema document: https://playground.open-rpc.org/?schemaUrl=https://raw.githubusercontent.com/ethereum/execution-apis/assembled-spec/openrpc.json")
	}

	return nil
}

func (s *JSONRPCSchema) ValidateResponse(method string, raw map[string]any, options ...bool) error {
	if s == nil {
		return nil
	}
	if s.requestSchemas[method] == nil {
		return nil
	}

	result, err := s.responseSchemas[method].Validate(gojsonschema.NewRawLoader(raw))
	if err != nil {
		return err
	}

	if !result.Valid() {
		errs := result.Errors()
		if revised := len(options) > 0 && options[0]; revised {
			errs = []gojsonschema.ResultError{}
			for i := 0; i < len(result.Errors()); i++ {
				if result.Errors()[i].Type() == "invalid_type" {
					field := result.Errors()[i].Field()
					if s._responseSchemas[method]["properties"] == nil || s._responseSchemas[method]["properties"].(map[string]any)[field] == nil {
						errs = append(errs, result.Errors()[i])
						continue
					}
					switch s._responseSchemas[method]["properties"].(map[string]any)[field].(map[string]any)["type"] {
					case "string":
						switch raw[field].(type) {
						case float64, float32:
							raw[field] = strconv.FormatFloat(raw[field].(float64), 'f', -1, 64)
						case int64, int32, int16, int8:
							raw[field] = strconv.FormatInt(raw[field].(int64), 16)
						default:
							raw[field] = fmt.Sprint(raw[field])
						}

						continue
					case "integer":
						switch raw[field].(type) {
						case float32, int64, int32, int16, int8:
							raw[field] = raw[field].(float64)
							continue
						default:
							if v, err := strconv.Atoi(fmt.Sprint(raw[field])); err == nil {
								raw[field] = v
								continue
							}
						}
					case "null":
						raw[field] = nil
						continue
					}
				}

				errs = append(errs, result.Errors()[i])
			}
		}

		if len(errs) > 0 {
			descriptions := slice.Map(errs, func(i int, err gojsonschema.ResultError) string {
				return "'" + err.Field() + "' " + err.Description()
			})
			return fmt.Errorf("%s result validate failed: %s", method, slice.Join(descriptions, "; "))
		}
	}

	return nil
}
