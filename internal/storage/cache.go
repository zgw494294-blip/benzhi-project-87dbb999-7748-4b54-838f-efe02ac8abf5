package storage

import (
	"sync"

	"cave-sampling-permit/internal/domain"
)

type aggregateCache struct {
	mu    sync.RWMutex
	items map[string]*domain.Application
}

func (c *aggregateCache) get(id string) *domain.Application {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.items[id]
}

func (c *aggregateCache) put(app *domain.Application) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[string]*domain.Application)
	}
	c.items[app.ID] = app
}

func (c *aggregateCache) remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, id)
}
