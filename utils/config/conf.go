package config

import (
	"log"
	"strconv"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/knadh/koanf/v2"
)

type Conf struct {
	*koanf.Koanf
}

func LoadEndpointChains(k *Conf, path string) {
	if !k.Exists(path) {
		return
	}

	var chains []common.EndpointChain
	err := k.Unmarshal(path, &chains)
	if err != nil {
		log.Panicf("Unmarshal chains error: %v", err)
	}

	for i := 0; i < len(chains); i++ {
		k.Set(helpers.Concat("chains.", strconv.FormatUint(chains[i].ChainID, 10)), chains[i])
		k.Set(helpers.Concat("chains.", chains[i].ChainCode), chains[i])
	}
}

func (c *Conf) Get(path string, defaultValues ...any) any {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Get(path)
}

func (c *Conf) Bool(path string, defaultValues ...bool) bool {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Bool(path)
}

func (c *Conf) String(path string, defaultValues ...string) string {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.String(path)
}

func (c *Conf) Strings(path string, defaultValues ...[]string) []string {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}
	return c.Koanf.Strings(path)
}

func (c *Conf) Int(path string, defaultValues ...int) int {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Int(path)
}

func (c *Conf) Int64(path string, defaultValues ...int64) int64 {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Int64(path)
}

func (c *Conf) Duration(path string, defaultValues ...time.Duration) time.Duration {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Duration(path)
}

func (c *Conf) Time(path string, layout string, defaultValues ...time.Time) time.Time {
	if !c.Koanf.Exists(path) && len(defaultValues) > 0 {
		return defaultValues[0]
	}

	return c.Koanf.Time(path, layout)
}

func (c *Conf) Copy() *Conf {
	return &Conf{Koanf: c.Koanf.Copy()}
}
