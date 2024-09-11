package common

type ChainId = uint64

type Chain struct {
	ID   uint64 `yaml:"id" json:"id"`
	Code string `yaml:"code" json:"code"`
}
