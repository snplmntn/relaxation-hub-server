package service

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type cacheEntry struct {
	result    *GeocodingResult
	timestamp time.Time
}

type CachedGeocoder struct {
	underlying Geocoder
	cache      *lru.Cache[string, *cacheEntry]
	ttl        time.Duration
}

func NewCachedGeocoder(underlying Geocoder, size int, ttl time.Duration) (Geocoder, error) {
	cache, err := lru.New[string, *cacheEntry](size)
	if err != nil {
		return nil, err
	}
	return &CachedGeocoder{
		underlying: underlying,
		cache:      cache,
		ttl:        ttl,
	}, nil
}

func (c *CachedGeocoder) Geocode(ctx context.Context, fullAddress string) (*GeocodingResult, error) {
	if entry, ok := c.cache.Get(fullAddress); ok {
		if time.Since(entry.timestamp) < c.ttl {
			return entry.result, nil
		}
	}

	result, err := c.underlying.Geocode(ctx, fullAddress)
	if err != nil {
		return nil, err
	}

	c.cache.Add(fullAddress, &cacheEntry{
		result:    result,
		timestamp: time.Now(),
	})

	return result, nil
}

func (c *CachedGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodingResult, error) {
	key := fmt.Sprintf("%f,%f", lat, lng)
	if entry, ok := c.cache.Get(key); ok {
		if time.Since(entry.timestamp) < c.ttl {
			return entry.result, nil
		}
	}

	result, err := c.underlying.ReverseGeocode(ctx, lat, lng)
	if err != nil {
		return nil, err
	}

	c.cache.Add(key, &cacheEntry{
		result:    result,
		timestamp: time.Now(),
	})

	return result, nil
}
