package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/module/core/rpc"
	"github.com/rs/zerolog"
)

type httpClientConfig struct {
	Transport     *http.Transport
	JSONRPCSchema *rpc.JSONRPCSchema
}

type httpClient struct {
	logger   zerolog.Logger
	endpoint *Endpoint
	client   *http.Client
	req      *http.Request
	config   *httpClientConfig
}

func NewHTTPClient(endpoint *Endpoint, config *httpClientConfig) Client {
	url := endpoint.Url().String()
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("name", "endpoint").Str("url", url).Logger()
	if url == "" {
		return nil
	}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		logger.Error().Msgf("Error creating request: %v", err)
		return nil
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if endpoint.Headers() != nil {
		for key, value := range endpoint.Headers() {
			headers.Set(key, value)
		}
	}
	req.Header = headers

	e := &httpClient{
		endpoint: endpoint,
		logger:   logger,
		client:   &http.Client{Transport: config.Transport},
		config:   config,
		req:      req,
	}

	return e
}

func (e *httpClient) request(ctx context.Context, b []byte) (*http.Response, common.HTTPErrors) {
	req := e.req.Clone(ctx)
	req.Body = io.NopCloser(bytes.NewBuffer(b))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBuffer(b)), nil
	}

	_EndpointGauge(e.endpoint).Inc()
	defer _EndpointGauge(e.endpoint).Dec()
	resp, err := e.client.Do(req)

	if err != nil {
		e.logger.Warn().Msgf("Sending request %s", b)
		e.logger.Error().Msgf("Error sending request: %v", err)

		if _err, ok := err.(*url.Error); ok {
			err = _err.Err
			if _err.Timeout() {
				return nil, common.TimeoutError(_err.Err.Error(), err)
			} else if _err.Err.Error() == "context deadline exceeded" {
				if cause := context.Cause(ctx); cause != nil {
					return nil, cause.(common.HTTPErrors)
				}
			}
		}

		return nil, common.UpstreamServerError("Error connection to endpoint", err)
	}

	return resp, nil
}

// Call implements Endpoint.
func (e *httpClient) Call(ctx context.Context, data []rpc.SealedJSONRPC, profiles ...*common.ResponseProfile) (results []rpc.JSONRPCResulter, err error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, common.InternalServerError("Marshalling request failed", err)
	}

	var profile = &common.ResponseProfile{}
	if len(profiles) > 0 {
		profile = profiles[0]
	}

	// 请求
	now := time.Now()
	resp, err := e.request(ctx, b)

	profile.Duration = time.Since(now).Milliseconds()

	defer updateMetrics(e.endpoint, profile)

	if err != nil {
		profile.Error = err.Error()
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.logger.Error().Msgf("Error sending request: %v", err)
		profile.Code = "connection_error"
		profile.Error = err.Error()
		return nil, common.UpstreamServerError("Reading response failed", err)
	}

	profile.Traffic = len(body)

	profile.Status = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		profile.Code = "http_error"
		e.logger.Debug().Msgf("HTTP status %d: %s", resp.StatusCode, string(body))
		return nil, common.UpstreamServerError("HTTP error", fmt.Errorf("status %d", resp.StatusCode))
	}

	results, isBatchResult, err := rpc.UnmarshalJSONRPCResults(body)
	if err != nil {
		e.logger.Warn().Msgf("Failed to unmarshal: %s", body)
		profile.Code = "request_error"
		profile.Error = err.Error()
		return nil, common.InternalServerError("Unmarshalling response failed", err)
	}

	if !isBatchResult && len(results) > 0 && results[0].Type() == rpc.JSONRPC_ERROR {
		recordingErrorResult(profile, results[0])
		return results, nil
	}

	// maybe is single result
	if len(results) != len(data) {
		return results, nil
	}

	if e.config.JSONRPCSchema != nil {
		if err := validateResults(e.logger, e.config.JSONRPCSchema, profile, data, results); err != nil {
			return nil, common.UpstreamServerError("Validating response failed", err)
		}
	}

	return results, nil
}

func (e *httpClient) Close() error {
	e.client.CloseIdleConnections()
	return nil
}
