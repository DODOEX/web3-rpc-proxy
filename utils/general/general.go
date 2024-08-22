package general

import "encoding/json"

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ByteLength(v any) int {
	switch v.(type) {
	case []byte:
		return len(v.([]byte))
	case string:
		return len([]byte(v.(string)))
	default:
		b, _ := json.Marshal(v)
		return len(b)
	}
}
