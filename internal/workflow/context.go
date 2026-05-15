package workflow

import (
	"encoding/json"
	"sync"
)

type RunContext struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewRunContext(seed map[string]any) *RunContext {
	data := map[string]any{}
	for k, v := range seed {
		data[k] = v
	}
	return &RunContext{data: data}
}

func (rc *RunContext) Get(key string) (any, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	v, ok := rc.data[key]
	return v, ok
}

func (rc *RunContext) Set(key string, val any) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.data[key] = val
}

func (rc *RunContext) Merge(updates map[string]any) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for k, v := range updates {
		rc.data[k] = v
	}
}

func (rc *RunContext) Clone() *RunContext {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	clone := make(map[string]any, len(rc.data))
	for k, v := range rc.data {
		clone[k] = v
	}
	return &RunContext{data: clone}
}

func (rc *RunContext) MarshalJSON() ([]byte, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return json.Marshal(rc.data)
}
