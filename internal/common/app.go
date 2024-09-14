package common

import (
	"strings"

	"github.com/DODOEX/web3rpcproxy/internal/app/database/schema"
	"github.com/knadh/koanf/maps"
)

type TenantInfo = schema.Tenant

type App struct {
	TenantInfo
	Bucket  string
	Balance int64
	// 用于流量防抖
	LastTime int64
	Offset   int64
}

func (a App) Preference(path string) any {
	value := a.Preferences.Get()
	if preferences, ok := value.(map[string]any); ok {
		p := strings.Split(path, ".")
		return maps.Search(preferences, p)
	}
	return nil
}

func (a App) HasPreference(path string) bool {
	if v := a.Preference(path); v != nil {
		return true
	}
	return false
}
