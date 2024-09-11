package helpers

func ToFloat(v any) (float64, bool) {
	switch v.(type) {
	case int:
		return float64(v.(int)), true
	case int8:
		return float64(v.(int8)), true
	case int16:
		return float64(v.(int16)), true
	case int32:
		return float64(v.(int32)), true
	case int64:
		return float64(v.(int64)), true
	case uint:
		return float64(v.(uint)), true
	case uint8:
		return float64(v.(uint8)), true
	case uint16:
		return float64(v.(uint16)), true
	case uint32:
		return float64(v.(uint32)), true
	case uint64:
		return float64(v.(uint64)), true
	case float32:
		return float64(v.(float32)), true
	case float64:
		return v.(float64), true
	}

	return 0.0, false
}
