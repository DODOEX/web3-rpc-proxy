package web3rpcprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/DODOEX/web3rpcproxy/internal/core/endpoint"
)

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

type Web3RPCProviderConfig struct {
	Sources []string
	Request *http.Request
	Client  *http.Client
}

type _rpc struct {
	ChainId uint64 `json:chainId`
	Url     string `json:url`
}

type Web3RPCProvider struct {
	Config *Web3RPCProviderConfig
}

func NewWeb3RPCProvider(config *Web3RPCProviderConfig) *Web3RPCProvider {
	if config.Client == nil {
		config.Client = &http.Client{}
	}
	p := &Web3RPCProvider{
		Config: config,
	}

	return p
}

func (p *Web3RPCProvider) Provide(ctx context.Context, chainIds ...uint64) (error, []*endpoint.Endpoint, bool) {
	req := p.Config.Request.Clone(ctx)

	query := url.Values{}
	for i := range p.Config.Sources {
		if i == 0 {
			query.Set("sources[]", p.Config.Sources[i])
		} else {
			query.Add("sources[]", p.Config.Sources[i])
		}
	}
	for i := range chainIds {
		if i == 0 {
			query.Set("chains[]", fmt.Sprint(chainIds[i]))
		} else {
			query.Add("chains[]", fmt.Sprint(chainIds[i]))
		}
	}
	req.URL.RawQuery = query.Encode()

	resp, err := p.Config.Client.Do(req)
	if err != nil {
		return err, nil, false
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, nil, false
	}

	rpcs := []_rpc{}
	json.Unmarshal(body, &rpcs)

	endpoints := []*endpoint.Endpoint{}

	if len(rpcs) <= 0 {
		return nil, endpoints, false
	}

	for i := range rpcs {
		parsedURL, err := url.Parse(rpcs[i].Url)
		if err != nil {
			continue
		}
		e := endpoint.New(parsedURL)
		e.Update(endpoint.WithAttr(endpoint.ChainId, rpcs[i].ChainId))
		endpoints = append(endpoints, e)
	}

	return nil, endpoints, true
}
