package shared

import (
	"os"
	"time"

	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp/prefork"
)

// initialize logger
func NewLogger(config *config.Conf) zerolog.Logger {
	zerolog.TimeFieldFormat = config.String("logger.time-format", time.RFC3339)

	if config.Bool("logger.prettier", true) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	l, err := zerolog.ParseLevel(config.String("logger.level", "info"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse log level")
	}
	zerolog.SetGlobalLevel(l)

	return log.Hook(PreforkHook{})
}

// prefer hook for zerologger
type PreforkHook struct{}

func (h PreforkHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if prefork.IsChild() {
		e.Discard()
	}
}
