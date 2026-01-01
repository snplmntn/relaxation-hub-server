package service

import (
	"sync"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ServiceCache provides in-memory caching for service queries with write-through invalidation.
// Cache persists until explicitly invalidated (no TTL).
type ServiceCache struct {
	active      []model.Service
	popular     []model.Service
	unavailable []model.Service
	activeOK    bool
	popularOK   bool
	unavailOK   bool
	mu          sync.RWMutex
}

// NewServiceCache creates a new service cache instance.
func NewServiceCache() *ServiceCache {
	return &ServiceCache{}
}

// GetActive returns cached active services, or fetches using fetchFn if cache is empty.
func (c *ServiceCache) GetActive(fetchFn func() ([]model.Service, error)) ([]model.Service, error) {
	c.mu.RLock()
	if c.activeOK {
		result := make([]model.Service, len(c.active))
		copy(result, c.active)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	// Fetch from DB
	services, err := fetchFn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.mu.Lock()
	c.active = services
	c.activeOK = true
	c.mu.Unlock()

	return services, nil
}

// GetPopular returns cached popular services, or fetches using fetchFn if cache is empty.
func (c *ServiceCache) GetPopular(fetchFn func() ([]model.Service, error)) ([]model.Service, error) {
	c.mu.RLock()
	if c.popularOK {
		result := make([]model.Service, len(c.popular))
		copy(result, c.popular)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	services, err := fetchFn()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.popular = services
	c.popularOK = true
	c.mu.Unlock()

	return services, nil
}

// GetUnavailable returns cached unavailable services, or fetches using fetchFn if cache is empty.
func (c *ServiceCache) GetUnavailable(fetchFn func() ([]model.Service, error)) ([]model.Service, error) {
	c.mu.RLock()
	if c.unavailOK {
		result := make([]model.Service, len(c.unavailable))
		copy(result, c.unavailable)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	services, err := fetchFn()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.unavailable = services
	c.unavailOK = true
	c.mu.Unlock()

	return services, nil
}

// Invalidate clears all cached data. Should be called when services are created/updated/deleted.
func (c *ServiceCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = nil
	c.popular = nil
	c.unavailable = nil
	c.activeOK = false
	c.popularOK = false
	c.unavailOK = false
}
