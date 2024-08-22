package config

import (
	"strings"
)

// func to parse address
func ParseAddress(raw string) (hostname, port string) {
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		return raw[:i], raw[i+1:]
	}

	return raw, ""
}
