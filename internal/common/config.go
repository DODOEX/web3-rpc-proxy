package common

type EndpointInfo struct {
	Url     string             `yaml:"url" koanf:"url" json:"url"`
	Headers *map[string]string `yaml:"headers" koanf:"headers" json:"headers"`
	Weight  *int               `yaml:"weight" koanf:"weight" json:"weight"`
}

type EndpointList = struct {
	Endpoints []*EndpointInfo `yaml:"list,omitempty" koanf:"list,omitempty"`
}

type EndpointServices = struct {
	Activenode EndpointList `yaml:"activenode" koanf:"activenode"`
	Fullnode   EndpointList `yaml:"fullnode" koanf:"fullnode"`
}

type EndpointChain = struct {
	ChainID   uint64 `yaml:"id" koanf:"id"`
	ChainCode string `yaml:"code" koanf:"code"`

	EndpointList `koanf:",omitempty,squash"`

	Services *EndpointServices `yaml:"services,omitempty" koanf:"services,omitempty"`
}
