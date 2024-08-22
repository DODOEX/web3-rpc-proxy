package shared

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/rs/zerolog"
)

func NewTransport(config *config.Conf, logger zerolog.Logger) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.WriteBufferSize = 32 * 1024
	t.ReadBufferSize = 32 * 1024
	t.MaxIdleConnsPerHost = http.DefaultMaxIdleConnsPerHost
	t.IdleConnTimeout = 30 * time.Second
	t.DisableCompression = false
	t.DisableKeepAlives = false
	t.TLSClientConfig.MinVersion = tls.VersionTLS10

	if err := config.Unmarshal("transport", t); err != nil {
		logger.Error().Err(err).Msg("failed to unmarshal transport config")
	}

	return t
}
