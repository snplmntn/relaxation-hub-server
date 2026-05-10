package service

import (
	"sync"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ServiceCache provides in-memory caching for service queries with TTL-based expiration.
// Default TTL is 5 minutes. Cache is also invalidated on write operations.
type ServiceCache struct {
	active      []model.Service
	popular     []model.Service
	unavailable []model.Service
	activeOK    bool
	popularOK   bool
	unavailOK   bool
	mu          sync.RWMutex
	ttl         time.Duration
	lastUpdate  time.Time
}

// NewServiceCache creates a new service cache instance with default 5-minute TTL.
func NewServiceCache() *ServiceCache {
	return &ServiceCache{
		ttl: 5 * time.Minute,
	}
}

// NewServiceCacheWithTTL creates a new service cache with custom TTL.
func NewServiceCacheWithTTL(ttl time.Duration) *ServiceCache {
	return &ServiceCache{
		ttl: ttl,
	}
}

// GetActive returns cached active services, or fetches using fetchFn if cache is empty or expired.
func (c *ServiceCache) GetActive(fetchFn func() ([]model.Service, error)) ([]model.Service, error) {
	c.mu.RLock()
	if c.activeOK && c.isValid() {
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
	c.lastUpdate = time.Now()
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
	c.lastUpdate = time.Time{} // Reset to zero time
}

// isValid checks if the cache is still within TTL (caller must hold read lock).
func (c *ServiceCache) isValid() bool {
	if c.ttl == 0 {
		return true // No TTL means cache never expires automatically
	}
	return !c.lastUpdate.IsZero() && time.Since(c.lastUpdate) < c.ttl
}
