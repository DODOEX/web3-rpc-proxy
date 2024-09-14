package shared

import (
	"context"

	"github.com/rs/zerolog"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type WatcherClient struct {
	logger      zerolog.Logger
	etcd        *clientv3.Client
	pathHandles map[string][]WatchHandler
}

func NewWatcherClientInstance(
	logger zerolog.Logger,
	etcd *clientv3.Client,
) *WatcherClient {
	return &WatcherClient{
		etcd:        etcd,
		logger:      logger.With().Str("name", "watcher").Logger(),
		pathHandles: make(map[string][]WatchHandler),
	}
}

type WatchHandler func(path string, value []byte)

func (w *WatcherClient) OnChanaged(path string, fn WatchHandler) {
	if w.pathHandles[path] == nil {
		w.pathHandles[path] = []WatchHandler{fn}
	} else {
		w.pathHandles[path] = append(w.pathHandles[path], fn)
	}
	go w.watch(path)
}

func (w *WatcherClient) watch(path string) {
	defer func() {
		if err := recover(); err != nil {
			w.logger.Error().Interface("error", err).Msgf("Failed to watch etcd")
		}
	}()

	ch := w.etcd.Watch(context.Background(), path)

	for resp := range ch {
		if w.pathHandles[path] == nil || len(w.pathHandles[path]) == 0 {
			continue
		}
		for i := range resp.Events {
			if resp.Events[i].Type == clientv3.EventTypePut && resp.Events[i].Kv.Value != nil {
				for i := range w.pathHandles[path] {
					w.pathHandles[path][i](path, resp.Events[i].Kv.Value)
				}
			}
		}
	}
}
