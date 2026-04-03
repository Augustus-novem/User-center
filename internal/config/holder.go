package config

import "sync/atomic"

type Holder struct {
	value atomic.Value
}

func NewHolder(cfg AppConfig) *Holder {
	h := &Holder{}
	h.value.Store(DynamicConfig{
		LogLevel: cfg.Log.Level,
		Feature:  cfg.Feature,
	})
	return h
}

func (h *Holder) Get() DynamicConfig {
	return h.value.Load().(DynamicConfig)
}

func (h *Holder) Update(cfg AppConfig) {
	h.value.Store(DynamicConfig{
		LogLevel: cfg.Log.Level,
		Feature:  cfg.Feature,
	})
}
