package api

import (
	"container/list"
	"sync"
)

type previewCacheEntry struct {
	key         string
	data        []byte
	contentType string
}

type lruPreviewCache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
}

func NewLRUPreviewCache(maxSize int) *lruPreviewCache {
	return &lruPreviewCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

func (c *lruPreviewCache) Get(key string) ([]byte, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, ""
	}
	c.order.MoveToFront(el)
	entry := el.Value.(*previewCacheEntry)
	return entry.data, entry.contentType
}

func (c *lruPreviewCache) Set(key string, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*previewCacheEntry).data = data
		return
	}
	entry := &previewCacheEntry{key: key, data: data, contentType: contentType}
	el := c.order.PushFront(entry)
	c.items[key] = el

	for c.order.Len() > c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.items, back.Value.(*previewCacheEntry).key)
	}
}

// fiber:context-methods migrated
