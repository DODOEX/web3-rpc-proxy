package endpoint

import (
	"github.com/duke-git/lancet/v2/maputil"
)

func Merge(vs ...*Endpoint) *Endpoint {
	// 如果有相同的键，后面 的值会覆盖 前面 的值
	result := make([]map[string]any, len(vs), cap(vs))
	for i := 0; i < len(vs); i++ {
		result[i] = vs[i].state
	}
	state := maputil.Merge(result...)

	return &Endpoint{state: state}
}
